/*
 * Copyright 2017 ETH Zurich
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

var selectedService;
var colorServiceSelect = "black";
var colorServiceDeselect = "gray";
var colorPaths = "black";
var colorSegCore = "red";
var colorSegUp = "green";
var colorSegDown = "blue";

// AS Topology Graph
var sv_link_dist = 75; // link distance
var ft_h = 14; // font height
var sv_tx_dy = 25; // service node label y-offset
var as_r = 130; // AS node radius
var sv_r = 11; // service node radius
var as_st = 6; // AS node stroke
var sv_st = 2; // service node stroke

/*
 * Creates on-click handler that will draw/hide selected path arcs.
 */
function setupPathSelection() {
    // handle path graph-only selection and color
    $("#as-iflist ul > li").click(function() {
        var type = $(this).attr("seg-type");
        var idx = parseInt($(this).attr("seg-num"));
        setPaths(type, idx, this.className == "open");
    });
}

function setPaths(type, idx, open) {
    if (open) {
        console.log(type + idx + ' opened');
        var num = idx + 1;
        if (type == 'CORE') {
            addSegments(resCore, idx, num, colorSegCore, type);
        } else if (type == 'DOWN') {
            addSegments(resDown, idx, num, colorSegDown, type);
        } else if (type == 'UP') {
            addSegments(resUp, idx, num, colorSegUp, type);
        } else if (type == 'PATH') {
            var latencies = getPathLatencyMin(formatPathString(resPath, idx,
                    type));
            addPaths(resPath, idx, num, colorPaths, type, latencies);
        }
        self.segType = type;
        self.segNum = idx;
    } else {
        console.log(type + idx + ' closed');
        removePaths();
        self.segType = undefined;
        self.segNum = undefined;
    }
}

/*
 * Produces a formatted path string for purposes of matching like: [1-ff00:0:111
 * 76>49 1-ff00:0:110 49>53 1-ff00:0:112]. Returns empty string for no path.
 */
function formatPathString(res, idx, type) {
    if (typeof idx === 'undefined') {
        return '';
    }
    var hops = res.if_lists[idx].interfaces;
    if (hops === 0 || type != 'PATH') {
        return '';
    }
    var path = "[";
    for (var i = 0; i < hops.length; i += 2) {
        var prev = hops[i];
        var next = hops[i + 1];
        if (i > 0) {
            path += ' ';
        }
        path += prev.ISD + '-' + prev.AS + ' ' + prev.IFID + '>' + next.IFID;
        if (i == (hops.length - 2)) {
            path += ' ' + next.ISD + '-' + next.AS;
        }
    }
    path += "]";
    return path;
}

function formatPathStringAll(res, type) {
    var paths = "";
    for (var i = 0; i < res.if_lists.length; i++) {
        var path = formatPathString(res, i, type);
        if (path != "") {
            if (i > 0) {
                paths += ",";
            }
            paths += formatPathString(res, i, type);
        }
    }
    return paths;
}

/*
 * Adds D3 forwarding path links with arrows and a title to paths graph.
 */
function addPaths(res, idx, num, color, type, latencies) {
    if (graphPath) {
        drawPath(res, idx, color, latencies);
        if (res.if_lists[idx].expTime) {
            drawTitle(type + ' ' + num, color, res.if_lists[idx].expTime);
        } else {
            drawTitle(type + ' ' + num, color);
        }
    }
    if (wv_map) {
        updateGMapAsLinks(res, idx, color);
    }
}

/*
 * Adds D3 segment links with arrows, title, and timer to paths graph.
 */
function addSegments(res, idx, num, color, type) {
    if (graphPath) {
        drawPath(res, idx, color);
        drawTitle(type + ' SEGMENT ' + num, color, res.if_lists[idx].expTime);
    }
    if (wv_map) {
        updateGMapAsLinks(res, idx, color);
    }
}

/*
 * Removes D3 path links, title, and timer from paths graph.
 */
function removePaths() {
    if (graphPath) {
        restorePath();
        removeTitle();
    }
    if (wv_map) {
        updateGMapAsLinks();
    }
}

/*
 * Test incoming segments for proper beaconing order, and invert when needed.
 * Since modifying up segments, can effect analysis of core segments, we analyze
 * in this order: paths, core, up, down.
 */
function orderPaths(src, dst) {
    // fwd paths should begin with src ia
    if (isIAHead(resPath, src)) {
        resPath.if_lists = invertInterfaces(resPath.if_lists);
    }
    if (resUp.if_lists.length > 0) {
        // if up segments, core should match one ia from up segments
        if (isCoreInverted(resCore, resUp)) {
            resCore.if_lists = invertInterfaces(resCore.if_lists);
        }
    } else {
        // if no up segments, core segments begin with src ia
        if (isIAHead(resCore, src)) {
            resCore.if_lists = invertInterfaces(resCore.if_lists);
        }
    }
    // up segments should begin with src ia
    if (isIAHead(resUp, src)) {
        resUp.if_lists = invertInterfaces(resUp.if_lists);
    }
    // down segments should end with dst ia
    if (isIATail(resDown, dst)) {
        resDown.if_lists = invertInterfaces(resDown.if_lists);
    }
}

/*
 * Utility to invert the hop order.
 */
function invertInterfaces(if_lists) {
    for (i in if_lists) {
        if_lists[i].interfaces.reverse();
    }
    return if_lists;
}

/*
 * Discover if IA is at the beginning of this set of segments.
 */
function isIAHead(segs, ia) {
    reverse = false;
    for (s in segs.if_lists) {
        head = segs.if_lists[s].interfaces[0];
        if ((head.ISD + '-' + head.AS) != ia) {
            reverse = true;
        }
    }
    return reverse;
}

/*
 * Discover if IA is at the end of this set of segments.
 */
function isIATail(segs, ia) {
    reverse = false;
    for (s in segs.if_lists) {
        tail = segs.if_lists[s].interfaces.slice(-1)[0];
        if ((tail.ISD + '-' + tail.AS) != ia) {
            reverse = true;
        }
    }
    return reverse;
}

/*
 * Discover if core segment heads can be found within up segments.
 */
function isCoreInverted(csegs, usegs) {
    reverse = true;
    for (c in csegs.if_lists) {
        head = csegs.if_lists[c].interfaces[0];
        for (u in usegs.if_lists) {
            for (i in usegs.if_lists[u].interfaces) {
                _if = usegs.if_lists[u].interfaces[i];
                if ((head.ISD + '-' + head.AS) == (_if.ISD + '-' + _if.AS)) {
                    reverse = false;
                }
            }
        }
    }
    return reverse;
}

/*
 * Handle open and close of data tree suggested by stackoverflow.com/a/38765843
 * and stackoverflow.com/a/38765843.
 */
function setupListTree() {
    var tree = document.querySelectorAll('ul.tree a:not(:last-child)');
    for (var i = 0; i < tree.length; i++) {
        tree[i].addEventListener('click', function(e) {
            e.preventDefault();
            var parent = e.target.parentElement;
            var classList = parent.classList;
            var closeAllOpenSiblings = function() {
                // MWF: ':scope .open' fails in MS IE/Edge, removed ':scope'
                var opensubs = parent.parentElement.querySelectorAll('.open');
                for (var i = 0; i < opensubs.length; i++) {
                    opensubs[i].classList.remove('open');
                }
            }
            if (classList.contains("open")) {
                classList.remove('open');
            } else {
                closeAllOpenSiblings();
                classList.add('open');
            }
        });
    }
}

/*
 * Translates incoming AS topology data to D3-compatible json.
 */
function parseTopo(topo) {
    var data = {};
    data.links = topo.links.map(function(value) {
        var nodes = topo.nodes;
        var link = {};
        link.type = value.type || 'default';
        for (i = 0; i < nodes.length; i++) {
            if (nodes[i].name === value.source) {
                link.source = i;
            }
            if (nodes[i].name === value.target) {
                link.target = i;
            }
        }
        return link;
    });
    data.nodes = topo.nodes;
    return data;
};

/*
 * Initializes AS topology graph, its handlers, and renders it.
 */
function drawAsTopo(div_id, json_as_topo, width, height) {

    var graphAs = parseTopo(json_as_topo);
    console.log(JSON.stringify(graphAs));

    var svgAs = d3.select("#" + div_id).append("svg").attr("height", height)
            .attr("width", width);

    var color = d3.scale.category10();
    var nodes = graphAs.nodes;
    var links = graphAs.links;

    var texts = svgAs.selectAll("text").data(nodes).enter().append("text")
            .attr("dy", sv_tx_dy).attr("text-anchor", "middle").attr("fill",
                    "black").attr("font-family", "sans-serif").attr(
                    "font-size", ft_h + "px").text(function(d) {
                return d.name;
            });

    var colaAs = cola.d3adaptor().size([ width, height ]).linkDistance(
            sv_link_dist).avoidOverlaps(true);
    colaAs.nodes(nodes).links(links).start(30)

    var edges = svgAs.selectAll("line").data(links).enter().append("line")
            .style("stroke-linecap", "round").attr("class", function(d) {
                return "link " + d.type;
            }).attr("marker-end", "url(#end)");

    var nodes = svgAs.selectAll("circle").data(nodes).enter().append("circle")
            .attr("r", function(d) {
                return (d.type == "root") ? as_r : sv_r - 1;
            }).on("click", onAsServiceClick).attr("opacity", function(d) {
                return 0.5;
            }).style("fill", function(d, i) {
                return (d.type == "root") ? "none" : color(d.group);
            }).style("stroke-width", function(d) {
                return (d.type == "root") ? as_st : sv_st;
            }).style("stroke", function(d) {
                return colorServiceDeselect;
            }).call(colaAs.drag);

    colaAs.on("tick", function() {

        edges.attr("x1", function(d) {
            return Math.max(sv_r, Math.min(width - sv_r, d.source.x));
        }).attr("y1", function(d) {
            return Math.max(sv_r, Math.min(height - sv_r - ft_h, d.source.y));
        }).attr("x2", function(d) {
            return Math.max(sv_r, Math.min(width - sv_r, d.target.x));
        }).attr("y2", function(d) {
            return Math.max(sv_r, Math.min(height - sv_r - ft_h, d.target.y));
        });

        nodes.attr("cx", function(d) {
            return d.x = Math.max(sv_r, Math.min(width - sv_r, d.x));
        }).attr("cy", function(d) {
            return d.y = Math.max(sv_r, Math.min(height - sv_r - ft_h, d.y));
        });

        texts.attr("transform", function(d) {
            return "translate(" + d.x + "," + d.y + ")";
        });
    });
}

/*
 * Creates handler to display AS topology service details per-click.
 */
function onAsServiceClick(d) {
    ser_name = d.name;
    document.getElementById("as-selection").innerHTML = ser_name;

    // display node details
    console.log(d);
    $('#service_table tbody > tr').remove();
    var k;
    var graph_vars = [ 'x', 'y', 'px', 'py', 'fixed', 'weight', 'index',
            'group', 'variable', 'bounds' ];
    for (k in d) {
        if (typeof d[k] !== 'function' && !graph_vars.includes(k)) {
            $('#service_table').find('tbody').append(
                    "<tr><td>" + k + "</td><td>" + d[k] + "</td></tr>");
        }
    }
    // allow root AS and all services to be highlighted
    if (!selectedService) {
        selectedService = this;
        updateNodeSelected(true, selectedService);
    } else {
        updateNodeSelected(false, selectedService);
        selectedService = this;
        updateNodeSelected(true, selectedService);
    }
}

/*
 * Maintains highlight status for selected service nodes.
 */
function updateNodeSelected(isSelected, selected) {
    d3.select(selected).style('stroke',
            isSelected ? colorServiceSelect : colorServiceDeselect);
    d3.select(selected).attr('opacity', isSelected ? 1 : 0.5);
}

var svc_pre = {
    0 : "unset", // Not set
    1 : "ds",
    2 : "cs",
    3 : "sb",
    4 : "sig",
    5 : "hps",
};

var svc_service = {
    0 : "unset",
    1 : "Discovery Service",
    2 : "Control Service",
    3 : "SIBRA",
    4 : "SIG",
    5 : "Hidden Path Service",
};

function get_json_as_topo(data, width, height) {
    var nodes = [];
    var links = [];
    var gidx = 0;
    if (typeof data.as_info === 'undefined') {
        return {
            "nodes" : [],
            "links" : []
        };
    }
    var a = data.as_info
    var iaf = a.IA.replace(/:/g, "_")

    // asinfo: add AS root node
    var asnode = {};
    asnode.name = a.IA;
    asnode.service = "ISD-AS";
    asnode.mtu = a.MTU;
    asnode.type = "root";
    asnode.group = gidx;
    // fix root in center
    asnode.x = width / 2;
    asnode.y = height / 2;
    asnode.fixed = true;
    nodes.push(asnode);

    var sinfos = data.svc_info;
    for (s in sinfos) {
        gidx += 1;
        // svcinfo: add all service nodes
        var snode = {};
        snode.name = svc_pre[s] + iaf
        snode.service = svc_service[s];
        snode.addr = sinfos[s]
        snode.type = "service";
        snode.group = gidx;
        nodes.push(snode);
        // svcinfo: add internal links to service nodes
        var slink = {};
        slink.source = asnode.name;
        slink.target = snode.name;
        slink.type = "as-in";
        links.push(slink);
    }

    // consider each unique ip/port as a single border router, which may
    // have multiple interfaces
    gidx += 1;
    var brs = {};
    var iinfos = data.if_info;
    for (i in iinfos) {
        ipv4 = iinfos[i].IP;
        port = iinfos[i].Port;
        zone = iinfos[i].Zone;

        var brkey = ipv4 + "-" + port + "-" + zone;
        if (!brs.hasOwnProperty(brkey)) {
            // ifinfo: add all interface BR nodes
            var snode = {};
            brs[brkey] = "br" + iaf + "-" + (Object.keys(brs).length + 1);
            snode.name = brs[brkey];
            snode.service = "Border Router";
            snode.ipv4 = ipv4;
            snode.port = port;
            snode.zone = zone;
            snode.type = "router";
            snode.group = gidx;
            nodes.push(snode);
            // ifinfo: add all interface BR links
            var slink = {};
            slink.source = asnode.name;
            slink.target = snode.name;
            slink.type = "as-in";
            links.push(slink);
        }

        // ifinfo: add all interface dest AS nodes
        var lnode = {};
        lnode.name = "(" + iinfos[i].IP + ")";
        lnode.service = "ISD-AS";
        lnode.type = "interface";
        lnode.link_type = "parent";
        lnode.group = 0;
        nodes.push(lnode);
        // ifinfo: add external links BR to dest AS nodes
        var llink = {};
        llink.source = brs[brkey];
        llink.target = lnode.name;
        llink.type = "as-parent";
        links.push(llink);
    }

    var graph = {};
    graph.nodes = nodes;
    graph.links = links;
    return graph;
}

function json2html(json, root) {
    var html = "<ul" + (root ? " class='tree'" : "") + ">";
    // var html = "<ul class='tree'>";
    jQuery.each(json, function(key, value) {
        html += "<li><a href='#'><b>" + key + "</b></a>";
        if (typeof value !== "object") {
            if (key.includes("Time")) {
                // make time readable
                var t = new Date(0);
                t.setUTCSeconds(value);
                html += ": " + t.toLocaleDateString() + " "
                        + t.toLocaleTimeString();
            } else {
                html += ": " + value;
            }
        } else {
            html += json2html(value, false);
        }
    })
    return html + "</ul>";
}

function showError(err) {
    if (err && err != '') {
        console.error(err);
        $("#as-error").html(err);
        $("#as-error").css('color', 'red');
    } else {
        $("#as-error").empty();
    }
}
