package transform

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/osmzen/filter"
)

// network represents a type of route network.
// prefix is what we should insert into
// the property we put on the feature (e.g: prefix + 'network' for
// 'bicycle_network' and so forth). shield_text_fn is a function called with the
// network and ref to get the text which should be shown on the shield.
type network struct {
	Type              string
	Prefix            string
	ShieldText        func(string, string) string
	NetworkImportance func(string, string) int
}

var footNetwork = network{
	Type:              "footNetwork",
	Prefix:            "walking_",
	ShieldText:        defaultShieldText,
	NetworkImportance: walkingNetworkImportance,
}

var busNetwork = network{
	Type:              "busNetwork",
	Prefix:            "bus_",
	ShieldText:        defaultShieldText,
	NetworkImportance: busNetworkImportance,
}

// Networks is the name -> details map for networks.
var networks = map[string]network{
	"road": network{
		Type:              "roadNetwork",
		Prefix:            "",
		ShieldText:        roadShieldText,
		NetworkImportance: roadNetworkImportance,
	},
	"foot":   footNetwork,
	"hiking": footNetwork,
	"bicycle": network{
		Type:              "bicycleNetwork",
		Prefix:            "bicycle_",
		ShieldText:        defaultShieldText,
		NetworkImportance: bicycleNetworkImportance,
	},
	"bus":        busNetwork,
	"trolleybus": busNetwork,
}

var networksByType = map[string]network{}

func init() {
	for _, n := range networks {
		networksByType[n.Type] = n
	}
}

// guessTypeFromNetwork
// Return a best guess of the type of network (road, hiking, bus, bicycle)
// from the network tag itself.
func guessTypeFromNetwork(network string) string {
	switch network {
	case "iwn", "nwn", "rwn", "lwn":
		return "hiking"
	case "icn", "ncn", "rcn", "lcn":
		return "bicycle"
	default:
		// hack for now - how can we tell bus routes from road routes?
		// it seems all bus routes are relations, where we have a route type
		// given, so this should default to roads.
		return "road"
	}
}

// mergeNetworksFromTags
// Take the network and ref tags from the feature and, if they both exist, add
// them to the mz_networks list. This is to make handling of networks and refs
// more consistent across elements.
func mergeNetworksFromTags(ctx *filter.Context, feature *geojson.Feature) {
	network := feature.Properties.MustString("network")
	ref := feature.Properties.MustString("ref")
	if network == "" || ref == "" {
		return
	}

	delete(feature.Properties, "network")
	delete(feature.Properties, "ref")

	mzNetworks, _ := feature.Properties["mz_networks"].([]string)
	mzNetworks = append(mzNetworks, guessTypeFromNetwork(network), network, ref)
	feature.Properties["mz_networks"] = mzNetworks
}

func extractNetworkInformation(ctx *filter.Context, feature *geojson.Feature) {
	// Take the triples of (route_type, network, ref) from `mz_networks` and
	// extract them into two arrays of network and shield_text information.

	mzNetworks, _ := feature.Properties["mz_networks"].([]string)
	delete(feature.Properties, "mz_networks")
	if len(mzNetworks) == 0 {
		return
	}

	// mzNetworks is a set of triples.
	// This is what the original tilezen/vector-datasource uses

	groups := map[string][][2]string{}
	for i := 0; i < len(mzNetworks); i += 3 {
		if n, ok := networks[mzNetworks[i]]; ok {
			groups[n.Type] = append(groups[n.Type], [2]string{mzNetworks[i+1], mzNetworks[i+2]})
		}
	}

	for nt, vals := range groups {
		network := networksByType[nt]

		allNetworks := "all_" + network.Prefix + "networks"
		allShieldTexts := "all_" + network.Prefix + "shield_texts"

		networkNames := make([]string, len(vals))
		shieldTexts := make([]string, len(vals))

		for i, val := range vals {
			name := val[0]
			ref := val[1]

			networkNames[i] = name
			shieldTexts[i] = network.ShieldText(name, ref)
		}

		feature.Properties[allNetworks] = networkNames
		feature.Properties[allShieldTexts] = shieldTexts
	}
}

type networkSorter struct {
	network  network
	networks []string
	shields  []string
}

func (ns networkSorter) Len() int {
	return len(ns.networks)
}

func (ns networkSorter) Swap(i, j int) {
	ns.networks[i], ns.networks[j] = ns.networks[j], ns.networks[i]
	ns.shields[i], ns.shields[j] = ns.shields[j], ns.shields[i]
}

func (ns networkSorter) Less(i, j int) bool {
	iv := ns.network.NetworkImportance(ns.networks[i], ns.shields[i])
	jv := ns.network.NetworkImportance(ns.networks[j], ns.shields[j])

	return iv < jv
}

// of all the (bike, road, etc.) networks this property is a member of,
// sort them by most important and then set the most important the main keys.
func sortNetworkProperties(properties geojson.Properties, network network) {
	// Use the `_network_importance` function to select any road networks from
	// `all_networks` and `all_shield_texts`, taking the most important one.

	allNetworks := "all_" + network.Prefix + "networks"
	allShields := "all_" + network.Prefix + "shield_texts"

	networks, _ := properties[allNetworks].([]string)
	delete(properties, allNetworks)

	shields, _ := properties[allShields].([]string)
	delete(properties, allShields)

	if len(networks) == 0 || len(shields) == 0 {
		return
	}

	ns := networkSorter{
		network:  network,
		networks: networks,
		shields:  shields,
	}

	sort.Sort(ns)

	properties[network.Prefix+"network"] = ns.networks[0]
	properties[network.Prefix+"shield_text"] = ns.shields[0]

	properties[allNetworks] = ns.networks
	properties[allShields] = ns.shields
}

func chooseMostImportantNetwork(ctx *filter.Context, feature *geojson.Feature) {
	for _, net := range networks {
		sortNetworkProperties(feature.Properties, net)
	}
}

func roadNetworkImportance(network, ref string) int {
	// Returns an integer representing the numeric importance of the network,
	// where lower numbers are more important.

	// This is to handle roads which are part of many networks, and ensuring
	// that the most important one is displayed. For example, in the USA many
	// roads can be part of both interstate (US:I) and "US" (US:US) highways,
	// and possibly state ones as well (e.g: US:NY:xxx). In addition, there
	// are international conventions around the use of "CC:national" and
	// "CC:regional:*" where "CC" is an ISO 2-letter country code.

	// Here we treat national-level roads as more important than regional or
	// lower, and assume that the deeper the network is in the hierarchy, the
	// less important the road. Roads with lower "ref" numbers are considered
	// more important than higher "ref" numbers, if they are part of the same
	// network.

	var nc int
	if network == "" {
		return 9999
	} else if network == "US:I" || strings.Contains(network, ":national") {
		nc = 1
	} else if network == "US:US" || strings.Contains(network, "regional") {
		nc = 2
	} else {
		nc = len(strings.Split(network, ":")) + 3

	}

	rc, _ := strconv.Atoi(ref)
	if rc < 0 {
		rc = 0
	}

	if rc > 9999 {
		rc = 9999
	}

	return nc*10000 + rc
}

var walkingNetworkCodes = map[string]int{
	"iwn": 1,
	"nwn": 2,
	"rwn": 3,
	"lwn": 4,
}

var bicycleNetworkCodes = map[string]int{
	"icn": 1,
	"ncn": 2,
	"rcn": 3,
	"lcn": 4,
}

func genericNetworkImportance(network, ref string, codes map[string]int) int {
	code := 0
	if codes != nil {
		// get a code based on the "largeness" of the network
		code = codes[network]
		if code == 0 {
			code = len(codes)
		}
	}

	if ref == "" {
		// Treat things with no ref as if they had a very high ref, and so reduced importance.
		return code*10000 + 9999
	}

	rc, err := strconv.Atoi(ref)
	if err != nil {
		// if ref isn't an integer, then it's likely a name, which might be
		// more important than a number
		// NOTE: will cause bus routes such as "39F" to be considered the most important.
		rc = 0
	}

	if rc < 0 {
		rc = 0
	}

	if rc > 9999 {
		rc = 9999
	}

	return code*10000 + rc
}

func walkingNetworkImportance(network, ref string) int {
	return genericNetworkImportance(network, ref, walkingNetworkCodes)
}

func bicycleNetworkImportance(network, ref string) int {
	return genericNetworkImportance(network, ref, bicycleNetworkCodes)
}

func busNetworkImportance(network, ref string) int {
	return genericNetworkImportance(network, ref, nil)
}

var numberAtFrontPattern = regexp.MustCompile(`^(\d+\w*)`)
var letterThenNumbersPattern = regexp.MustCompile(`(?i)^[^\d\s_]+[ -]?([\d]+)`)
var uaTerritorialPattern = regexp.MustCompile(`(?i)^(\w)-(\d+)-(\d+)$`)

func roadShieldText(network, ref string) string {
	// Try to extract the string that should be displayed within the road shield,
	// based on the raw ref and the network value.
	if ref == "" {
		return ""
	}

	// FI-PI-LI is just a special case?
	if ref == "FI-PI-LI" {
		return ref
	}

	// These "belt" roads have names in the ref which should be in the shield,
	// there's no number.
	if network == "US:PA:Belt" {
		return ref
	}

	// Ukranian roads sometimes have internal dashes which should be removed.
	if strings.HasPrefix(network, "ua:") {
		matches := uaTerritorialPattern.FindStringSubmatch(ref)
		if len(matches) != 0 {
			return matches[1] + matches[2] + matches[3]
		}
	}

	// Greek roads sometimes have alphabetic prefixes which we should _keep_,
	// unlike for other roads.
	if strings.HasPrefix(network, "GR:") || strings.HasPrefix(network, "gr:") {
		return ref
	}

	// If there's a number at the front (optionally with letters following),
	// then that's the ref.
	matches := numberAtFrontPattern.FindStringSubmatch(ref)
	if len(matches) != 0 {
		return matches[1]
	}

	// Otherwise, try to match a bunch of letters followed by a number.
	matches = letterThenNumbersPattern.FindStringSubmatch(ref)
	if len(matches) != 0 {
		return matches[1]
	}

	// Failing that, give up and just return the ref as-is.
	return ref
}

func defaultShieldText(network, ref string) string {
	// Without any special properties of the ref to make the shield text from,
	// just use the 'ref' property.
	return ref
}
