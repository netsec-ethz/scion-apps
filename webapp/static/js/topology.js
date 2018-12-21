/*
 * Copyright 2016 ETH Zurich
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

var LinkType = {
    Core : 'CORE',
    Parent : 'PARENT',
    Peer : 'PEER',
    Child : 'CHILD',
};

var ISDAS = new RegExp("^[0-9]+-[0-9a-fA-F_:\/]+$");
var ISD = new RegExp("^[0-9]+");
var AS = new RegExp("[0-9a-fA-F_:\/]+$");

/*
 * Retrieve given ID because the link will consist of source-target or
 * target-source.
 */
function getPathId(source, target) {
    var res = 'null';

    // search source - target
    for (var i = 0; i < graphPath.links.length; i++) {
        if (graphPath.links[i].source.name == source) {
            if (graphPath.links[i].target.name == target) {
                res = "source_" + source + "_target_" + target;
            }
        }
    }

    // and target - source
    for (var j = 0; j < graphPath.links.length; j++) {
        if (graphPath.links[j].target.name == source) {
            if (graphPath.links[j].source.name == target) {
                res = "source_" + target + "_target_" + source;
            }
        }
    }
    return res;
}

/*
 * Placeholder nodes ensure dark/light node color patterns will be consistent
 * and always shift the color scheme by 2: 1 core, 1 not-core.
 */
function addPlaceholderNode(graph, isd, core) {
    var name = isd + "-" + (parseInt(isd) + 100 + core);
    var group = ((ISD.exec(name) - 1) * 4) + core;
    graph["nodes"].push({
        name : name,
        group : group,
        type : "placeholder"
    });
}

/*
 * Sorting method for nodes that will ensure consistent grouping alignment when
 * D3 uses a color map.
 */
function sortTopologyGraph(graph) {
    // add placeholder nodes for consistent coloring
    var isds = [];
    for (var n = 0; n < graph.nodes.length; n++) {
        var isd = graph.nodes[n].name.split("-")[0];
        if (!isds.includes(isd)) {
            isds.push(isd);
        }
    }
    isds.sort();
    for (var n = 0; n < isds.length; n++) {
        addPlaceholderNode(graph, isds[n], 0);
        addPlaceholderNode(graph, isds[n], 1);
    }
    // sort for optimal color coding display
    graph.nodes.sort(function(a, b) {
        // node sort order: placeholder type, node group id, ISD#, AS#, is core
        var ph = (a.type !== "placeholder") - (b.type !== "placeholder");
        var grp = a.group - b.group;
        var isd = ISD.exec(a.name) - ISD.exec(b.name);
        var core = (a.type != LinkType.Core) - (b.type != LinkType.Core);
        var as = AS.exec(a.type) - AS.exec(b.type);
        if (ph != 0)
            // sorting placeholders first allows colors to be deterministic
            // since otherwise d3 will enumerate colors by default
            return ph;
        if (grp != 0)
            // sorting by group number next corrects some member object order
            // issues with Chrome where members would occasionally invert
            // between odd/even color groupings effectively making coloring for
            // core/non-core inconsistent for d3
            return grp;
        if (isd != 0)
            // ISDs should all be grouped together, in numerical order
            return isd;
        if (as != 0)
            // ASes should be listed in numerical order
            return as;
        if (core != 0)
            // ASes should consistently be ordered core first to allow the
            // darkest shades for core, and lighter for non-core
            return core;
        return 0; // default
    });
    // adjust indexes to match
    for (var n = 0; n < graph.nodes.length; n++) {
        graph.ids[graph.nodes[n].name] = n;
    }
}

/*
 * Helper method for adding unique AS nodes for paths graph.
 */
function addNodeFromLink(graph, name, type, node) {
    var core = (type.toLowerCase() === "core") ? 0 : 1;
    var group = ((ISD.exec(name) - 1) * 4) + core;
    graph["nodes"].push({
        name : name,
        group : group,
        type : type
    });
    graph["ids"][name] = node;
}

/*
 * Modifies existing path topology with more accurate segments, but only when
 * available.
 */
function updateGraphWithSegments(graph, segs, head) {
    for (var i = 0; i < segs.length; i++) {
        if (head) {
            _if = segs[i].interfaces[0];
        } else {
            _if = segs[i].interfaces.slice(-1)[0];
        }
        core_ia = _if.ISD + "-" + _if.AS;
        for (var j = 0; j < graph.nodes.length; j++) {
            var name = graph.nodes[j].name;
            if (name == core_ia) {
                graph.nodes[j].group = ((ISD.exec(name) - 1) * 4);
                graph.nodes[j].type = LinkType.Core;
            }
        }
    }
}

/*
 * Converts links-only topology data into D3 graph layout.
 */
function convertLinks2Graph(links_topo) {
    var graph = {
        nodes : [],
        links : [],
        ids : {}
    };
    var node = 0;
    for (var i = 0; i < links_topo.length; i++) {
        if (!(links_topo[i].a in graph["ids"])) {
            addNodeFromLink(graph, links_topo[i].a, links_topo[i].ltype, node);
            node++;
        }
        if (!(links_topo[i].b in graph["ids"])) {
            addNodeFromLink(graph, links_topo[i].b, links_topo[i].ltype, node);
            node++;
        }
    }
    sortTopologyGraph(graph);
    for (var i = 0; i < links_topo.length; i++) {
        if (links_topo[i].a != links_topo[i].b) {
            graph["links"].push({
                source : graph["ids"][links_topo[i].a],
                target : graph["ids"][links_topo[i].b],
                type : links_topo[i].ltype
            });
        }
    }
    return graph;
}

/*
 * Converts multipath-demo topology data into D3 graph layout.
 */
function convertTopo2Graph(topology) {
    var graph = {
        nodes : [],
        links : [],
        ids : {}
    };
    var i = 0;
    var key;
    for (key in topology) {
        if (ISDAS.test(key)) {
            var type = topology[key]["level"].toLowerCase();
            var core = (type == "core") ? 0 : 1;
            var group = ((ISD.exec(key) - 1) * 4) + core;

            graph["nodes"].push({
                name : key,
                group : group,
                type : type
            });
            graph["ids"][key] = i;
            i++;
        }
    }
    sortTopologyGraph(graph);
    var source;
    for (source in topology) {
        if (ISDAS.test(source)) {
            var target;
            for (target in topology[source]["links"]) {
                graph["links"].push({
                    source : graph["ids"][source],
                    target : graph["ids"][target],
                    type : "normal"
                });
            }
        }
    }
    return graph;
}
