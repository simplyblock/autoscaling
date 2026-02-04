//go:build linux

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// vxlan interface details
	VXLAN_IF_NAME     = "neon-vxlan0"
	VXLAN_BRIDGE_NAME = "neon-br0"
	VXLAN_ID          = 100

	// iptables settings details
	iptablesChainName = "NEON-EXTRANET"
	extraNetCidr      = "10.100.0.0/16"
)

var deleteIfaces = flag.Bool("delete", false, `delete VXLAN interfaces`)

func main() {
	flag.Parse()

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	// -delete option used for teardown vxlan setup
	if *deleteIfaces {
		log.Printf("deleting vxlan interface %s", VXLAN_IF_NAME)
		if err := deleteLink(VXLAN_IF_NAME); err != nil {
			log.Print(err)
		}
		log.Printf("deleting bridge interface %s", VXLAN_BRIDGE_NAME)
		if err := deleteLink(VXLAN_BRIDGE_NAME); err != nil {
			log.Print(err)
		}
		log.Printf("deleting iptables nat rules")
		if err := deleteIptablesRules(); err != nil {
			log.Print(err)
		}
		os.Exit(0)
	}

	ownNodeIP := os.Getenv("MY_NODE_IP")
	log.Printf("own node IP: %s", ownNodeIP)

	// create linux bridge
	log.Printf("creating linux bridge interface (name: %s)", VXLAN_BRIDGE_NAME)
	if err := createBrigeInterface(VXLAN_BRIDGE_NAME); err != nil {
		log.Fatal(err)
	}

	// configure bridge IP
	log.Printf("configuring IP for bridge %s based on node IP %s", VXLAN_BRIDGE_NAME, ownNodeIP)
	if err := configureBridgeIP(VXLAN_BRIDGE_NAME, ownNodeIP); err != nil {
		log.Fatal(err)
	}

	// create vxlan
	log.Printf("creating vxlan interface (name: %s, id: %d)", VXLAN_IF_NAME, VXLAN_ID)
	if err := createVxlanInterface(VXLAN_IF_NAME, VXLAN_ID, ownNodeIP, VXLAN_BRIDGE_NAME); err != nil {
		log.Fatal(err)
	}

	for {
		log.Print("getting nodes IP addresses")
		nodeIPs, err := getNodesIPs(clientset)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("found %d ip addresses", len(nodeIPs))

		// update FDB
		log.Print("update FDB table")
		if err := updateFDB(VXLAN_IF_NAME, nodeIPs, ownNodeIP); err != nil {
			log.Fatal(err)
		}
		// upsert iptables nat rules
		log.Printf("upsert iptables nat rules")
		if err := upsertIptablesRules(); err != nil {
			log.Print(err)
		}
		time.Sleep(30 * time.Second)
	}
}

func getNodesIPs(clientset *kubernetes.Clientset) ([]string, error) {
	ips := []string{}
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return ips, err
	}
	for _, n := range nodes.Items {
		for _, a := range n.Status.Addresses {
			if a.Type == corev1.NodeInternalIP {
				ips = append(ips, a.Address)
			}
		}
	}
	return ips, nil
}

func createBrigeInterface(name string) error {
	// check if interface already exists
	link, err := netlink.LinkByName(name)
	if err == nil {
		log.Printf("link with name %s already found, applying settings", name)
	} else {
		_, notFound := err.(netlink.LinkNotFoundError) //nolint:errorlint // errors.Is doesn't work, we actually just want to know the type.
		if !notFound {
			return err
		}

		// create an configure linux bridge
		link = &netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: name,
			},
		}
		if err := netlink.LinkAdd(link); err != nil {
			return err
		}
	}

	// Apply critical network settings (bridge-nf, offloading)
	if err := applyBridgeNetworkSettings(name); err != nil {
		log.Printf("warning: failed to apply bridge settings: %v", err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return err
	}

	return nil
}

func createVxlanInterface(name string, vxlanID int, ownIP string, bridgeName string) error {
	// check if interface already exists
	link, err := netlink.LinkByName(name)
	if err == nil {
		log.Printf("link with name %s already found, applying settings", name)
	} else {
		_, notFound := err.(netlink.LinkNotFoundError) //nolint:errorlint // errors.Is doesn't work, we actually just want to know the type.
		if !notFound {
			return err
		}

		// create an configure vxlan
		link = &netlink.Vxlan{
			LinkAttrs: netlink.LinkAttrs{
				Name: name,
				MTU:  1410, // Adjust MTU to account for VXLAN overhead on 1460 host MTU
			},
			VxlanId:  vxlanID,
			SrcAddr:  net.ParseIP(ownIP),
			Port:     4789,
			Learning: true,
		}

		if err := netlink.LinkAdd(link); err != nil {
			return err
		}

		// add vxlan to bridge
		br, err := netlink.LinkByName(bridgeName)
		if err != nil {
			return err
		}
		if err := netlink.LinkSetMaster(link, br); err != nil {
			return err
		}
	}

	// Set MTU even if interface already exists (in case it was wrong)
	if err := netlink.LinkSetMTU(link, 1410); err != nil {
		log.Printf("warning: failed to set MTU on %s: %v", name, err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return err
	}

	// Disable offloading (vxlan specific)
	if err := disableNicOffloading(name); err != nil {
		log.Printf("warning: failed to disable offloading on %s: %v", name, err)
	}

	return nil
}

func updateFDB(vxlanName string, nodeIPs []string, ownIP string) error {
	broadcastFdbMac, _ := net.ParseMAC("00:00:00:00:00:00")

	// get vxlan interface details
	link, err := netlink.LinkByName(vxlanName)
	if err != nil {
		return err
	}

	for _, ip := range nodeIPs {
		if ip != ownIP {
			if net.ParseIP(ip).To4() == nil {
				log.Printf("not adding IPv6 addr %q to FDB broadcast entry, no support for it", ip)
				continue
			}

			broadcastFdbEntry := netlink.Neigh{
				LinkIndex:    link.Attrs().Index,
				Family:       syscall.AF_BRIDGE,
				State:        netlink.NUD_PERMANENT,
				Flags:        netlink.NTF_SELF,
				IP:           net.ParseIP(ip),
				HardwareAddr: broadcastFdbMac,
			}
			// add entry to FDB table
			// duplicate append action will not case error.
			log.Printf("add/update FDB broadcast entry via %s", ip)
			if err := netlink.NeighAppend(&broadcastFdbEntry); err != nil {
				return err
			}
		}
	}

	return nil
}

func deleteLink(name string) error {
	// check if interface already exists
	link, err := netlink.LinkByName(name)
	if err == nil {
		if err := netlink.LinkDel(link); err != nil {
			return err
		}
		log.Printf("link with name %s was deleted", name)
		return nil
	}
	_, notFound := err.(netlink.LinkNotFoundError) //nolint:errorlint // errors.Is doesn't work, we actually just want to know the type.
	if !notFound {
		return err
	}
	log.Printf("link with name %s not found", name)

	return nil
}

func upsertIptablesRules() error {
	// manage iptables
	ipt, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv4), iptables.Timeout(5))
	if err != nil {
		return err
	}
	chainExists, err := ipt.ChainExists("nat", iptablesChainName)
	if err != nil {
		return err
	}
	if !chainExists {
		err := ipt.NewChain("nat", iptablesChainName)
		if err != nil {
			return err
		}
	}

	if err := insertRule(ipt, "nat", "POSTROUTING", 1, "-d", extraNetCidr, "-j", iptablesChainName); err != nil {
		return err
	}
	if err := insertRule(ipt, "nat", iptablesChainName, 1, "-s", extraNetCidr, "-d", extraNetCidr, "-j", "RETURN"); err != nil {
		return err
	}
	if err := insertRule(ipt, "nat", iptablesChainName, 2, "-d", extraNetCidr, "!", "-s", extraNetCidr, "-j", "MASQUERADE"); err != nil {
		return err
	}
	if err := insertRule(ipt, "nat", iptablesChainName, 3, "-s", extraNetCidr, "-j", "ACCEPT"); err != nil {
		return err
	}

	// Ensure FORWARD chain allows traffic.
	// In some K8s setups, default policy is DROP or there are reject rules.
	// We insert an unconditional ACCEPT at the top of the FORWARD chain.
	// We use "filter" table (default).
	iptFilter, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv4), iptables.Timeout(5))
	if err != nil {
		return err
	}
	// Insert ACCEPT rule at position 1.
	if err := insertRule(iptFilter, "filter", "FORWARD", 1, "-j", "ACCEPT"); err != nil {
		// Log error but don't fail hard if filter table access has issues, though it should work.
		log.Printf("warning: failed to insert FORWARD ACCEPT rule: %v", err)
	}

	return nil
}

func deleteIptablesRules() error {
	// manage iptables
	ipt, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv4), iptables.Timeout(5))
	if err != nil {
		return err
	}
	err = ipt.ClearAndDeleteChain("nat", iptablesChainName)
	if err != nil {
		return err
	}

	return nil
}

// insertRule acts like Insert except that it won't insert a duplicate (no matter the position in the chain)
func insertRule(ipt *iptables.IPTables, table, chain string, pos int, rulespec ...string) error {
	exists, err := ipt.Exists(table, chain, rulespec...)
	if err != nil {
		return err
	}

	if !exists {
		return ipt.Insert(table, chain, pos, rulespec...)
	}

	return nil
}

func configureBridgeIP(bridgeName string, nodeIP string) error {
	// Parse the node IP
	parsedNodeIP := net.ParseIP(nodeIP)
	if parsedNodeIP == nil {
		return &net.ParseError{Type: "IP address", Text: nodeIP}
	}
	ipv4 := parsedNodeIP.To4()
	if ipv4 == nil {
		// fallback or error for IPv6 if not supported in this logic
		// simpler to error out or skip for now if only IPv4 logic applies
		// For now assuming IPv4 as per task context
		log.Printf("warning: node IP %s is not a standard IPv4 address", nodeIP)
	}

	// Strategy: Use 10.100.255.<last_octet>/16
	// This ensures it falls into the free upper range of the 10.100.0.0/16 subnet
	lastOctet := ipv4[3]
	newIP := fmt.Sprintf("10.100.255.%d/16", lastOctet)

	log.Printf("calculating overlay IP: %s (derived from %s)", newIP, nodeIP)

	addr, err := netlink.ParseAddr(newIP)
	if err != nil {
		return err
	}

	link, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	// Check if the address implies a route (it does)
	// We use AddrReplace to update/set it
	if err := netlink.AddrReplace(link, addr); err != nil {
		return err
	}

	return nil
}

func applyBridgeNetworkSettings(name string) error {
	// 1. Attempt to disable bridge-nf on this specific bridge via ip link attributes.
	// This is the safer, localized way to prevent iptables interaction, though it might not work on all kernels.
	// We DO NOT disable the global sysctl as that breaks cluster CNIs (like Flannel) that rely on it.
	cmd := exec.Command("ip", "link", "set", "dev", name, "type", "bridge", "nf_call_iptables", "0", "nf_call_ip6tables", "0", "nf_call_arptables", "0")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("warning: failed to disable localized bridge-nf on %s: %v, output: %s", name, err, string(out))
	}

	// 2. Disable offloading on the bridge interface
	// Prevents checksum/segmentation offload issues on the virtual bridge.
	return disableNicOffloading(name)
}

func disableNicOffloading(name string) error {
	// Disables TX/RX checksumming and segmentation offloads
	cmd := exec.Command("ethtool", "-K", name, "tx", "off", "rx", "off", "tso", "off", "gso", "off", "gro", "off", "lro", "off")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ethtool failed: %w, output: %s", err, string(out))
	}
	return nil
}
