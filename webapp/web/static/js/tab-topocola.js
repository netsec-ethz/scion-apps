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

var graphPath;
var colorPath;
var colaPath;
var svgPath;

var setup = {};
var colors = {};

var source_added = false;
var destination_added = false;

var possible_colors = {
    "red" : "#ff0000",
    "green" : "#008000",
    "blue" : "#0000ff",
    "yellow" : "#ffff00",
    "purple" : "#800080",
    "black" : "#222222",
    "none" : "#ffffff",
}

// Paths graph
var default_link_color = "#999999";
var default_link_opacity = "0.35";
var p_link_dist = 70; // paths link distance
var p_link_dist_long = 80; // long paths link distance
var p_r = 23; // path node radius
var pt_w = 90; // text node rect width
var pt_h = 35; // text node rect height
var pt_r = 4; // text node rect corner radius
var pl_w = 15; // path legend width
var ph_h = 25; // path title header height
var ph_m = 5; // path title header margin
var ph_p = ph_h + ph_m; // path title header padding
var short_as_node = true; // true: circle, short AS; false: oval, long AS

var pageBounds;
var circlesg;
var linesg;

// Timer
var expClock;
var secondMs = 1000;
var minuteMs = secondMs * 60;
var hourMs = minuteMs * 60;
var yearMs = hourMs * 24;

/*
 * Post-rendering method to add labels to paths graph. The position of these
 * text anchors should change with the relative number of as nodes to be
 * increasingly farther apart and into the bottom corners to encourage complex
 * paths to spread out for better viewing.
 */
function topoSetup(msg, width, height) {
    setup = {};
    if (graphPath == undefined) {
        console.error("No graphPath to add setup!!");
        return;
    }
    for (key in msg) {
        setup[key] = msg[key];
    }

    // use smallest path to find min links for basis of anchors
    var paths = resPath.if_lists;
    if (paths.length == 0) {
        return;
    }
    var min_interfaces = paths[0].interfaces.length;
    for (path in resPath.if_lists) {
        if (paths[path].interfaces.length < min_interfaces) {
            min_interfaces = paths[path].interfaces.length;
        }
    }
    var min_link = (min_interfaces / 2) - 1;
    // min placement from center for smallest topology (s=src, d=dst)
    var min_sx = (width - pt_w) / 2;
    var min_dx = (width + pt_w) / 2;
    var min_y = ((height + pt_h) / 2) + p_link_dist;
    // max placement from center for largest topology (max bounds)
    var max_sx = (pt_w / 2);
    var max_dx = width - (pt_w / 2);
    var max_y = height - (pt_h / 2);
    // optimal placement to center based on number of links
    var opt_sx = min_sx - (min_link * p_link_dist / 2);
    var opt_dx = min_dx + (min_link * p_link_dist / 2);
    var opt_y = min_y + (min_link * p_link_dist / 2);
    // choose optimal placement, else do not exceed max bounds
    if (msg.hasOwnProperty("destination") && !destination_added) {
        addFixedLabel("destination", (opt_dx < max_dx ? opt_dx : max_dx),
                (opt_y < max_y ? opt_y : max_y), false);
        destination_added = true;
    }
    if (msg.hasOwnProperty("source") && !source_added) {
        addFixedLabel("source", (opt_sx > max_sx ? opt_sx : max_sx),
                (opt_y < max_y ? opt_y : max_y), true);
        source_added = true;
    }
}

/*
 * Post rendering method to assign colors to path node links and labels.
 */
function topoColor(msg) {
    colors = {};
    for (key in msg) {
        colors[key] = msg[key];

        acceptable_paths = [ "path1", "path2", "path3" ];

        if (acceptable_paths.indexOf(key) != -1) {
            var prev = "none";

            for (i in setup[key]) {
                if (prev != "none") {
                    updatePathProperties(prev, setup[key][i], msg[key]);
                }
                prev = setup[key][i];
            }
        }
    }

    if (msg.hasOwnProperty("source")) {
        $(".node.source").attr("fill", possible_colors[msg["source"]]);
    }

    if (msg.hasOwnProperty("destination")) {
        $(".node.destination")
                .attr("fill", possible_colors[msg["destination"]]);
    }
}

/*
 * Updates path visibility and style properties.
 */
function updatePathProperties(prevPath, currPath, color) {
    if (color != "none") {
        $(".source-" + prevPath + ".target-" + currPath).attr("stroke",
                possible_colors[color]).attr("stroke-opacity", "1");

        $(".source-" + currPath + ".target-" + prevPath).attr("stroke",
                possible_colors[color]).attr("stroke-opacity", "1");
    } else {
        $(".source-" + prevPath + ".target-" + currPath).attr("stroke",
                default_link_color)
                .attr("stroke-opacity", default_link_opacity);

        $(".source-" + currPath + ".target-" + prevPath).attr("stroke",
                default_link_color)
                .attr("stroke-opacity", default_link_opacity);
    }
}

/*
 * Initializes paths graph, its handlers, and renders it.
 */
function drawTopology(div_id, original_json_data, segs, width, height) {
    source_added = false;
    destination_added = false;
    graphPath = null;
    colorPath = null;
    colaPath = null;
    svgPath = null;
    setup = {};
    colors = {};
    circlesg = null;
    linesg = null;

    if (original_json_data.length == 0) {
        console.error("No data to draw topology!!");
        return;
    }

    console.log(JSON.stringify(original_json_data));
    graphPath = convertLinks2Graph(original_json_data);

    // first node in each up and down segment must be core
    updateGraphWithSegments(graphPath, segs.up_segments.if_lists, false);
    updateGraphWithSegments(graphPath, segs.down_segments.if_lists, true);

    console.log(JSON.stringify(graphPath));
    colorPath = d3.scale.category20();
    p_link_dist = short_as_node ? p_link_dist : p_link_dist_long;
    colaPath = cola.d3adaptor().jaccardLinkLengths(p_link_dist)
            .convergenceThreshold(1e-3).avoidOverlaps(true).handleDisconnected(
                    true).size([ width, height ]);

    svgPath = d3.select("#" + div_id).append("svg").attr("width", width).attr(
            "height", height);

    pageBounds = {
        x : 0,
        y : 0,
        width : width,
        height : height
    };

    // Arrow marker
    svgPath.append("defs").selectAll("marker").data(
            [ colorPaths, colorSegCore, colorSegDown, colorSegUp ]).enter()
            .append("marker").attr("id", function(d) {
                return d;
            }).attr("viewBox", "0 -5 10 10").attr("refX", p_r + 10).attr(
                    "refY", -5).attr("markerWidth", 6).attr("markerHeight", 6)
            .attr("orient", "auto").append("path").attr("d", "M0,-5L10,0L0,5")
            .attr('fill', function(d, i) {
                return d
            });

    linesg = svgPath.append("g");
    pathsg = svgPath.append("g");
    circlesg = svgPath.append("g");

    update();
    topoColor({
        "source" : "none",
        "destination" : "none",
        "path1" : "red",
        "path2" : "green",
        "path3" : "blue"
    });
    drawLegend();
}

/*
 * Calculates the constraints properties used by webcola to render initial graph
 * inside window boundaries.
 */
function calcConstraints(realGraphNodes) {
    var topLeft = {
        x : pageBounds.x,
        y : pageBounds.y,
        fixed : true
    };
    var tlIndex = graphPath.nodes.push(topLeft) - 1;
    var bottomRight = {
        x : pageBounds.x + pageBounds.width,
        y : pageBounds.y + pageBounds.height,
        fixed : true
    };
    var brIndex = graphPath.nodes.push(bottomRight) - 1;
    var constraints = [];

    for (var i = 0; i < realGraphNodes.length; i++) {
        constraints.push({
            axis : 'x',
            type : 'separation',
            left : tlIndex,
            right : i,
            gap : p_r
        });
        constraints.push({
            axis : 'y',
            type : 'separation',
            left : tlIndex,
            right : i,
            gap : p_r
        });
        constraints.push({
            axis : 'x',
            type : 'separation',
            left : i,
            right : brIndex,
            gap : p_r
        });
        constraints.push({
            axis : 'y',
            type : 'separation',
            left : i,
            right : brIndex,
            gap : p_r
        });
    }
    return constraints;
}

/*
 * Paths graph update method to iterate over all nodes and links when changes
 * occur like, adding or removing path arcs.
 */
function update() {
    var realGraphNodes = graphPath.nodes.slice(0);

    var constraints = calcConstraints(realGraphNodes);
    colaPath.constraints(constraints).links(graphPath.links).nodes(
            graphPath.nodes)

    var path = linesg.selectAll("path.link").data(graphPath.links)
    path.enter().append("path").attr("class", function(d) {
        var src = "source-" + d.source.name;
        var dst = "target-" + d.target.name;
        return d.type + " link " + src + " " + dst;
    }).attr("stroke", default_link_color).attr("stroke-opacity",
            default_link_opacity);
    path.exit().remove();

    var markerLinks = graphPath.links.filter(function(link) {
        return link.path;
    });

    // remove previous markers to prevent color bleeding
    svgPath.selectAll("path.marker").remove();

    var markerPath = pathsg.selectAll("path.marker").data(markerLinks)
    markerPath.enter().append("path").attr("class", function(d) {
        return "marker " + d.type;
    }).attr("marker-end", function(d) {
        return "url(#" + d.color + ")";
    }).style("stroke", function(d) {
        return d.color;
    });
    markerPath.exit().remove();

    var node = circlesg.selectAll(".node").data(realGraphNodes, function(d) {
        return d.name;
    })
    var nodeg = node.enter().append("g").attr("class", function(d) {
        return "node";
    }).attr("id", function(d) {
        return "node_" + d.name;
    }).call(colaPath.drag).attr("transform", nodeTransform);

    nodeg.append("rect").attr("width", function(d) {
        if (short_as_node) {
            return (d.type == "host") ? pt_w : (2 * p_r)
        } else {
            return (d.type == "host") ? pt_w : labelWidth(d.name);
        }
    }).attr("height", function(d) {
        return (d.type == "host") ? pt_h : (2 * p_r)
    }).attr("rx", function(d) {
        return (d.type == "host") ? pt_r : p_r
    }).attr("ry", function(d) {
        return (d.type == "host") ? pt_r : p_r
    }).attr("x", function(d) {
        if (short_as_node) {
            return -((d.type == "host") ? pt_w / 2 : p_r)
        } else {
            return -((d.type == "host") ? pt_w : labelWidth(d.name)) / 2;
        }
    }).attr("y", function(d) {
        return -((d.type == "host") ? pt_h / 2 : p_r)
    }).style("fill", function(d) {
        return (d.type == "host") ? "white" : colorPath(d.group);
    }).style("visibility", function(d) {
        return (d.type == "placeholder") ? "hidden" : "visible";
    }).attr("stroke", default_link_color);

    nodeg.append("text").attr("text-anchor", "middle").attr("y", ".35em").attr(
            "class", function(d) {
                return d.type + " label";
            }).text(function(d) {
        if (isNodeShortened(d)) {
            return shortenNode(d);
        } else {
            return d.name;
        }
    }).style("visibility", function(d) {
        return (d.type == "placeholder") ? "hidden" : "visible";
    });

    // tooltip for long AS names
    node.on("mouseover", function(d) {
        var cbName = $('#switch_as_names').prop('checked');
        var cbNumber = $('#switch_as_numbers').prop('checked');
        if (!cbName && !cbNumber) {
            var g = d3.select(this);
            g.append('text').classed('tool', true)
                    .attr("text-anchor", "middle").attr('y', -p_r - ph_m)
                    .style("font-size", "12px").text(function(d) {
                        return getNodeInfoText(d, true, true);
                    });
        }
    })
    node.on("mouseout", function() {
        d3.select(this).select('text.tool').remove();
    });

    node.exit().remove();

    colaPath.on("tick", function(d) {

        path.attr("d", linkStraight);
        markerPath.attr("d", linkArc);
        node.attr("transform", nodeTransform);
    });

    colaPath.start(50, 100, 200);
}

/*
 * Determine best info/tooltip display of text.
 */
function getNodeInfoText(d, useNumber, useName) {
    var isAs = (d.type != "host" && d.type != "placeholder");
    var asLabel = '';
    if (useNumber) {
        asLabel += isNodeShortened(d) ? d.name : '';
    }
    if (useName && iaLabels && iaLabels.AS && iaLabels.AS[d.name]) {
        if (asLabel.length > 0) {
            asLabel += ' ';
        }
        asLabel += iaLabels.AS[d.name];
    }
    return isAs ? asLabel : '';
}

/*
 * Determines if name of node has been shortened.
 */
function isNodeShortened(d) {
    var reISDAS = new RegExp(/^[0-9]+-[0-9a-f]+:[0-9a-f]+:[0-9a-f]+$/i);
    return (short_as_node && reISDAS.test(d.name) && d.type != "host");
}

/*
 * Shorten ISD-AS DD-HHHH:HHHH:HHHH to HHHH:HHHH.
 */
function shortenNode(d) {
    var isdas = d.name.split('-');
    if (isdas.length == 2) {
        var as = isdas[1].split(':');
        if (as.length == 3) {
            return as[1] + ':' + as[2];
        }
    }
    return d.name;
}

/*
 * Gets pixel width of node based on label size, includes radius as basis of
 * minumim width.
 */
function labelWidth(label) {
    calc = label.length * 6.5;
    return calc > (2 * p_r) ? calc : (2 * p_r);
}

/*
 * Returns the SVG instructions for rendering a path arc as a straight line.
 */
function linkStraight(d) {
    var yh = p_r + ph_p;
    var x1 = Math.max(p_r, Math.min(pageBounds.width - p_r, d.source.x));
    var y1 = Math.max(yh, Math.min(pageBounds.height - p_r, d.source.y));
    var x2 = Math.max(p_r, Math.min(pageBounds.width - p_r, d.target.x));
    var y2 = Math.max(yh, Math.min(pageBounds.height - p_r, d.target.y));

    var dr = 0;
    return "M" + x1 + "," + y1 + "A" + dr + "," + dr + " 0 0,1 " + x2 + ","
            + y2;
}

/*
 * Returns the SVG instructions for rendering a path arc.
 */
function linkArc(d) {
    var yh = p_r + ph_p;
    var x1 = Math.max(p_r, Math.min(pageBounds.width - p_r, d.source.x));
    var y1 = Math.max(yh, Math.min(pageBounds.height - p_r, d.source.y));
    var x2 = Math.max(p_r, Math.min(pageBounds.width - p_r, d.target.x));
    var y2 = Math.max(yh, Math.min(pageBounds.height - p_r, d.target.y));

    var dx = x2 - x1;
    var dy = y2 - y1;
    var dr = Math.sqrt(dx * dx + dy * dy);
    return "M" + x1 + "," + y1 + "A" + dr + "," + dr + " 0 0,1 " + x2 + ","
            + y2;
}

/*
 * Creates the instructions for positioning each node.
 */
function nodeTransform(d) {
    var yh = p_r + ph_p;
    var dx = Math.max(p_r, Math.min(pageBounds.width - p_r, d.x));
    var dy = Math.max(yh, Math.min(pageBounds.height - p_r, d.y));
    return "translate(" + dx + "," + dy + ")";
}

/*
 * Build legend labels based only on visible nodes.
 */
function buildLabels() {
    var shown = [];
    for (var n = 0; n < graphPath.nodes.length; n++) {
        // if type not placeholder, add to shown
        if (graphPath.nodes[n].type != "placeholder") {
            shown.push(graphPath.nodes[n].group);
        }
    }
    return shown;
}

/*
 * Renders the paths legend and color key. Labels should not be "shown" in the
 * legend if they are not in the actual topology being displayed.
 */
function drawLegend() {
    var shown = buildLabels();
    var idx = 0;
    var legend = svgPath.selectAll(".legend").data(colorPath.domain()).enter()
            .append("g").attr("class", "legend").attr("transform", function(d) {
                // Use our enumerated idx for labels, not all-color index i
                var y = 0;
                if (shown.includes(d)) {
                    y = ph_m + (idx * (pl_w + 2));
                    idx++;
                }
                return "translate(0," + y + ")";
            });
    legend.append("rect").attr("x", ph_m).attr("width", pl_w).attr("height",
            pl_w).style("fill", colorPath).style("visibility", function(d) {
        return shown.includes(d) ? "visible" : "hidden";
    });
    var x_offset = (ph_m * 2) + pl_w;
    var y_offset = pl_w / 2;
    legend.append("text").attr("x", x_offset).attr("y", y_offset).attr("dy",
            ".35em").style("text-anchor", "begin").style("font-size", "12px")
            .text(function(d) {
                if (shown.includes(d)) {
                    var core = '';
                    var label = '';
                    if (d % 2 === 0) {
                        isd = d / 4 + 1;
                        core = ' (core)';
                    } else {
                        isd = (d - 1) / 4 + 1;
                    }
                    if (iaLabels && iaLabels.ISD && iaLabels.ISD[String(isd)]) {
                        label = ' ' + iaLabels.ISD[String(isd)];
                    }
                    return 'ISD-' + isd + label + core;
                } else {
                    return null;
                }
            });
}

/*
 * Post-rendering method to add a label attached to a fixed point on the graph.
 */
function addFixedLabel(label, x, y, lastLabel) {
    // remove last 2 constraint nodes from the end first
    if (!lastLabel) {
        graphPath.nodes.pop();
        graphPath.nodes.pop();
    }

    // update graph elements with additions
    graphPath["ids"][label] = Object.keys(graphPath["ids"]).length;
    graphPath.nodes.push({
        name : label,
        group : 1,
        type : "host",
        x : x,
        y : y,
        fixed : true,
    });
    if (graphPath["ids"][setup[label]]) {
        graphPath.links.push({
            source : graphPath["ids"][setup[label]],
            target : graphPath["ids"][label],
            type : "host",
        });
    }

    // redraw graph, recalculating constraints
    if (lastLabel) {
        update();
    }
}

/*
 * Post-rendering method to draw path arcs for the given path and color.
 */
function drawPath(res, path, color) {
    // get the index of the routes to render
    var routes = [];
    if (path < 0) {
        for (var i = 0; i < res.if_lists.length; i++) {
            routes.push(i);
        }
    } else {
        routes.push(path);
    }
    var path_ids = [];
    for (var p = 0; p < routes.length; p++) {
        var pNum = parseInt(routes[p]);
        // select the target path, and make iteration as amount of how many
        for (var ifNum = 0; ifNum < res.if_lists[pNum].interfaces.length; ifNum++) {
            var ifRes = res.if_lists[pNum].interfaces[ifNum];
            path_ids.push(ifRes.ISD + '-' + ifRes.AS);
        }
    }

    topoSetup({
        "path1" : path_ids
    });
    topoColor({
        "path1" : color
    });

    graphPath.nodes.pop();
    graphPath.nodes.pop();
    // reset
    graphPath.links = graphPath.links.filter(function(link) {
        return !link.path;
    });
    for (var i = 0; i < path_ids.length - 1; i++) {
        // prevent src == dst links from being formed
        if (path_ids[i] != path_ids[i + 1]) {
            graphPath.links.push({
                "color" : color,
                "path" : true,
                "source" : graphPath["ids"][path_ids[i]],
                "target" : graphPath["ids"][path_ids[i + 1]],
                "type" : "PARENT"
            });
        }
    }
    update();
}

/*
 * Removes all path arcs from the graph.
 */
function restorePath() {

    topoSetup({
        "path1" : []
    });
    topoColor({
        "path1" : "none"
    });

    graphPath.nodes.pop();
    graphPath.nodes.pop();
    // reset
    graphPath.links = graphPath.links.filter(function(link) {
        return !link.path;
    });
    update();
}

/*
 * Adds title and countdown to path graph.
 */
function drawTitle(title, color, expTime) {
    width = svgPath.attr("width");
    height = svgPath.attr("height");
    removeTitle();
    svgPath.append("text").attr("class", "title").attr("x", width / 2).attr(
            "y", ph_h).attr("text-anchor", "middle").style("font-size", "18px")
            .style("text-weight", "bold").style("fill", color).text(title);

    if (expTime) {
        remain = (expTime * 1000) - Date.now();
        var days = Math.floor(remain / yearMs);
        var hours = Math.floor((remain % yearMs) / hourMs);
        var minutes = Math.floor((remain % hourMs) / minuteMs);
        var seconds = Math.floor((remain % minuteMs) / secondMs);
        var clock = [];
        clock.push(('00' + hours).substr(-2), ('00' + minutes).substr(-2),
                ('00' + seconds).substr(-2));
        clockStr = (days > 0 ? days + "d " : "") + clock.join(":");
        remainStr = remain > 0 ? clockStr : "Expired.";
        svgPath.append("text").attr("class", "clock").attr("x", width - ph_m)
                .attr("y", ph_h).attr("text-anchor", "end").style("font-size",
                        "14px").style("fill", color).text(remainStr);
        if (remain > 0) {
            expClock = setTimeout(function() {
                drawTitle(title, color, expTime)
            }, secondMs)
        }
    }
}

/*
 * Clears title and countdown elements from path graph.
 */
function removeTitle() {
    clearTimeout(expClock);
    svgPath.selectAll(".title").remove();
    svgPath.selectAll(".clock").remove();
}
