// Copyright 2019 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.package main

var resSegs, resCore, resUp, resDown, resPath, jTopo, json_as_topo;
var iaLabels;
var iaLocations = [];
var iaGeoLoc;
var g = {};
var jPathColors = [];

function setupDebug(src, dst) {
    var src = $('#ia_cli').val();
    var dst = $('#ia_ser').val();
    g['src'] = src;
    g['dst'] = dst;
    var isd = parseInt(src.split('-')[0]);
    if (isd <= 15) {
        g['debug'] = true; // test ISD found, set debug
    }
}

var cMissingPath = '#cccccc';
var cPaths = [ "#3366cc", "#dc3912", "#ff9900", "#109618", "#990099",
        "#0099c6", "#dd4477", "#66aa00", "#b82e2e", "#316395", "#994499",
        "#22aa99", "#aaaa11", "#6633cc", "#e67300", "#8b0707", "#651067",
        "#329262", "#5574a6", "#3b3eac" ];
function path_colors(n) {
    return cPaths[n % cPaths.length];
}

function getPathColor(hops) {
    var idx = jPathColors.indexOf(hops + '');
    if (idx < 0) {
        return cMissingPath;
    } else {
        return path_colors(idx);
    }
}

function isConfigComplete(data, textStatus, jqXHR) {
    console.log(JSON.stringify(data));
    g['nodes_xml_url'] = data.nodes_xml_url;
    g['labels_json_url'] = data.labels_json_url;
    g['google_mapsjs_apikey'] = data.google_mapsjs_apikey;
    g['google_geolocation_apikey'] = data.google_geolocation_apikey;

    // request labels/locations and wait to call view until done
    $.when(ajaxLabels({
        debug : g.debug,
        labels_json_url : g.labels_json_url,
    }), ajaxLocations({
        debug : g.debug,
        nodes_xml_url : g.nodes_xml_url,
    }), ajaxGeoLocate({
        debug : g.debug,
        google_geolocation_apikey : g.google_geolocation_apikey,
    })).done(function(aLbls, aLocs, aGeo) {
        isLabelsComplete(aLbls[0], aLbls[1], aLbls[2]);
        isLocationsComplete(aLocs[0], aLocs[1], aLocs[2]);
        isGeolocateComplete(aGeo[0], aGeo[1], aGeo[2]);
        loadPathData(g.src, g.dst);
    });
}

/*
 * If labels are found, translate to new AS numbering if needed.
 */
function isLabelsComplete(data, textStatus, jqXHR) {
    console.log(JSON.stringify(data));
    iaLabels = data; // global availablity
    // allow AS names labels option based on availablity of labels
    var showNames = iaLabels && iaLabels.ISD;
    $('#div_as_names').css("display", showNames ? "inline-block" : "none");
    $('#div_as_numbers').css("display", true ? "inline-block" : "none");
}

function ajaxConfig() {
    return $.ajax({
        url : 'config',
        type : 'get',
        dataType : "json",
        data : g,
        timeout : 30000,
        success : isConfigComplete,
        error : function(jqXHR, textStatus, errorThrown) {
            showError(this.url + ' ' + textStatus + ': ' + errorThrown);
        },
    });
}

function ajaxLabels(data) {
    return $.ajax({
        url : 'labels',
        type : 'get',
        dataType : "json",
        data : data,
        timeout : 10000,
        error : function(jqXHR, textStatus, errorThrown) {
            showError(this.url + ' ' + textStatus + ': ' + errorThrown);
        },
    });
}

function ajaxLocations(data) {
    return $.ajax({
        url : 'locations',
        type : 'get',
        dataType : "xml",
        data : data,
        timeout : 10000,
        error : function(jqXHR, textStatus, errorThrown) {
            showError(this.url + ' ' + textStatus + ': ' + errorThrown);
        },
    });
}

function ajaxGeoLocate(data) {
    return $.ajax({
        url : 'geolocate',
        type : 'get',
        dataType : "json",
        data : data,
        timeout : 15000,
        success : isGeolocateComplete,
        error : function(jqXHR, textStatus, errorThrown) {
            showError(this.url + ' ' + textStatus + ': ' + errorThrown);
        },
    });
}

function isGeolocateComplete(data, textStatus, jqXHR) {
    console.log(JSON.stringify(data));
    iaGeoLoc = data.location;
}

function drawMap(src, dst, local) {
    // generate list of ia markers from returned paths
    self.jLoc = [];// global availablity
    var ourIAs = [];
    for (var p = 0; p < resPath.if_lists.length; p++) {
        for (var i = 0; i < resPath.if_lists[p].interfaces.length; i++) {
            var ia = resPath.if_lists[p].interfaces[i];
            if (ISD.test(ia.ISD) && AS.test(ia.AS)) {
                var isdas = ia.ISD + '-' + ia.AS;
                if (!ourIAs.includes(isdas)) {
                    ourIAs.push(isdas);
                    var iaLocs = $.grep(iaLocations, function(e, i) {
                        return e.ia === isdas;
                    });
                    if (iaLocs.length > 0) {
                        // each ia found, use location
                        for (var l = 0; l < iaLocs.length; l++) {
                            self.jLoc.push({
                                ia : isdas,
                                lat : iaLocs[l].lat,
                                lng : iaLocs[l].lng,
                                host : iaLocs[l].host,
                            });
                        }
                    } else if (src == isdas) {
                        // we can only expect src to geolocate
                        self.jLoc.push({
                            ia : isdas,
                            lat : local.lat,
                            lng : local.lng,
                            host : "Origin IP Address",
                        });
                    }
                    // remaining "unknown" locations, do not render
                }
            }
        }
    }
    // setup map with path ISDs
    var isds = [];
    for (key in self.jLoc) {
        var isdAs = self.jLoc[key];
        var iface = isdAs.ia.split("-");
        var isd = parseInt(iface[0]);
        if (isds.map(function(e) {
            return e.ia;
        }).indexOf(isd) === -1) {
            isds.push({
                ia : isd,
                label : iaLabels ? iaLabels.ISD[iface[0]] : '',
            });
        }
    }

    var wait = setInterval(function() {
        console.warn('waited 500ms');
        wv_map = document.getElementById('g-map');
        console.log('got iframe:', wv_map);
        if (wv_map.contentWindow) {
            clearInterval(wait);
            initGMap(isds, g.google_mapsjs_apikey);

            // setup map with known markers
            updateGMapAsMarkers(src, dst);

            // don't add links since it gets too messy.
            // to restore all possible links uncomment the next line
            // updateGMapAsLinksAll();

            var cbName = $('#switch_as_names').prop('checked');
            var cbNumber = $('#switch_as_numbers').prop('checked');
            updateGMapAsLabels(cbName, cbNumber);

            handleAsLabelSwitch();
            var open = typeof self.segType !== 'undefined';
            setPaths(self.segType, self.segNum, open);
        }
    }, 500);
}

/*
 * Marks missing locations in paths dropdown tree list.
 */
function highlightNoGeoCode(src) {
    var ourIAs = [];
    for (var p = 0; p < resPath.if_lists.length; p++) {
        for (var i = 0; i < resPath.if_lists[p].interfaces.length; i++) {
            var ia = resPath.if_lists[p].interfaces[i];
            var isdas = ia.ISD + '-' + ia.AS;
            if (!ourIAs.includes(isdas)) {
                ourIAs.push(isdas);
            }
        }
    }

    var geoLocIAs = [];
    $.grep(iaLocations, function(e, i) {
        geoLocIAs.push(e.ia);
    });
    geoLocIAs.push(src); // our src IA is geolocatable
    var notGeoLocIAs = $.grep(ourIAs, function(el) {
        return $.inArray(el, geoLocIAs) == -1
    });
    notGeoLocIAs.forEach(function(ia) {
        dt = "data-toggle='tooltip'";
        t = "title='" + ia + " unknown map location'";
        str = "<b " + dt + " " + t + ">" + ia + "*</b>";
        $("#as-iflist li:contains(" + ia + ")").html(function(_, html) {
            return html.split(ia).join(str);
        });
    });
}

/*
 * Update static into labels based on checkboxes. Update paths based on selected
 * path state.
 */
function handleAsLabelSwitch() {
    var cbName = $('#switch_as_names').prop('checked');
    var cbNumber = $('#switch_as_numbers').prop('checked');
    var topoMap = $('#radio_pathMap').prop('checked');
    if (topoMap) {
        updateGMapAsLabels(cbName, cbNumber);
    } else {
        var g = d3.selectAll(".node");
        g.select('text.info').remove(); // clean old labels first
        text = g.append('text').classed('info', true)
        text.attr("text-anchor", "middle").attr('y', -p_r - ph_m).style(
                "font-size", "12px").text(function(g) {
            return getNodeInfoText(g, cbNumber, cbName);
        });
    }
}

/*
 * If locations are found, translate to new AS numbering if needed.
 */
function isLocationsComplete(data, textStatus, jqXHR) {
    // recieve list of known IA locations
    var xml_node = $('nodes', data);
    iaLocations = [];
    $.each(xml_node.find('node'), function() {
        iaLocations.push({
            ia : $(this).attr('ia'),
            lat : $(this).attr('lat'),
            lng : $(this).attr('long'),
            host : $(this).attr('host'),
        });
    });
    console.log(JSON.stringify(iaLocations));
}

/*
 * Final preparation, to hide/show info checkboxes and draw legend.
 */
function prepareInfoCheckBoxes() {
    // allow AS names labels option based on availablity of labels
    var showNames = iaLabels && iaLabels.ISD;
    $('#div_as_names').css("display", showNames ? "inline-block" : "none");
    $('#div_as_numbers').css("display", true ? "inline-block" : "none");
}

function handleMapTopologySwitch(topologyUpdate) {
    var htmlGMap = "<iframe id='g-map' src='./static/html/map.html' frameborder='0'></iframe>";
    var topoMap = $('#radio_pathMap').prop('checked');
    console.log("map checked", topoMap);
    if (topologyUpdate) {
        // for new topology, reset each view
        if ($("#as-pathtopo").children("svg").length != 0) {
            // topo svg should redraw from scratch
            $("#as-pathtopo").children("svg").remove();
            drawTopo(g.src, g.dst, jTopo, resSegs);
        }
        if ($("#as-pathtopo").children("iframe").length != 0) {
            // map should reuse its previous canvas to prevent reloads
            drawMap(g.src, g.dst, iaGeoLoc);
        }
    }
    if (topoMap) {
        $("#as-pathtopo").children("svg").hide();
        // only load map/geolocate once, to prevent exessive quota loads
        if ($("#as-pathtopo").children("iframe").length == 0) {
            $("#as-pathtopo").append(htmlGMap);
            // get potential locations
            drawMap(g.src, g.dst, iaGeoLoc);
        } else {
            $("#as-pathtopo").children("iframe").show();
            handleAsLabelSwitch();
        }
    } else {
        $("#as-pathtopo").children("iframe").hide();
        // only load svg once
        if ($("#as-pathtopo").children("svg").length == 0) {
            // load path topology
            drawTopo(g.src, g.dst, jTopo, resSegs);
        } else {
            $("#as-pathtopo").children("svg").show();
        }
        handleAsLabelSwitch();
    }
}

function drawTopo(src, dst, paths, segs) {
    var width = $("#as-pathtopo").width();
    var height = $("#as-pathtopo").height();
    drawTopology("as-pathtopo", paths, segs, width, height);
    // add endpoint labels
    var open = typeof self.segType !== 'undefined';
    setPaths(self.segType, self.segNum, open);
    topoSetup({
        "source" : src,
        "destination" : dst,
    }, width, height);
}

function formatPathJson(paths, idx) {
    if (typeof idx === 'undefined') {
        return '';
    }
    var ent = paths[idx].Entry;
    var hops = ent.Path.Interfaces;
    var path = "[";
    for (var i = 0; i < hops.length; i += 2) {
        var prev = hops[i];
        var next = hops[i + 1];
        if (i > 0) {
            path += ' ';
        }
        path += iaRaw2Read(prev.RawIsdas) + ' ' + prev.IfID + '>' + next.IfID;
        if (i == (hops.length - 2)) {
            path += ' ' + iaRaw2Read(next.RawIsdas);
        }
    }
    path += "]";
    return path;
}

function get_path_html(paths, csegs, usegs, dsegs, show_segs) {
    var html = "<ul class='tree'>";
    for (p in paths) {

        var ent = paths[p].Entry;
        var exp = new Date(0);
        if_ = ent.Path.Interfaces;
        var hops = if_.length / 2;

        var style = "style='background-color: "
                + getPathColor(formatPathJson(paths, parseInt(p))) + "; '";
        html += "<li seg-type='PATH' seg-num=" + p + "><a " + style
                + " href='#'>PATH " + (parseInt(p) + 1)
                + "</a> <span class='badge'>" + hops + "</span>";
        exp.setUTCSeconds(ent.Path.ExpTime);
        html += "<ul>";
        html += "<li><a href='#'>Mtu: " + ent.Path.Mtu + "</a>";
        if (ent.HostInfo.Addrs.Ipv4) {
            html += "<li><a href='#'>Ipv4: "
                    + ipv4Raw2Read(ent.HostInfo.Addrs.Ipv4) + "</a>";
        }
        if (ent.HostInfo.Addrs.Ipv6) {
            html += "<li><a href='#'>Ipv6: "
                    + ipv6Raw2Read(ent.HostInfo.Addrs.Ipv6) + "</a>";
        }
        html += "<li><a href='#'>Port: " + ent.HostInfo.Port + "</a>";
        html += "<li><a href='#'>Expiration: " + exp.toLocaleDateString() + " "
                + exp.toLocaleTimeString() + "</a>";
        for (i in if_) {
            html += "<li><a href='#'>" + iaRaw2Read(if_[i].RawIsdas) + " ("
                    + if_[i].IfID + ")</a>";
        }
        html += "</ul>";
    }
    if (show_segs) {
        html += get_segment_info(csegs, "CORE");
        html += get_segment_info(usegs, "UP");
        html += get_segment_info(dsegs, "DOWN");
    }
    html += "</ul>";
    return html;
}

// add style to list of paths and segments
function getSegColor(type) {
    if (type == "CORE") {
        return colorSegCore;
    } else if (type == "DOWN") {
        return colorSegDown;
    } else if (type == "UP") {
        return colorSegUp;
    } else {
        return colorPaths;
    }
}

function get_segment_info(segs, type) {
    var html = "";
    for (s in segs.if_lists) {
        var exp = new Date(0);
        exp.setUTCSeconds(segs.if_lists[s].expTime);
        if_ = segs.if_lists[s].interfaces;
        var hops = if_.length / 2;
        var style = "style='color: " + getSegColor(type) + ";'";
        html += "<li seg-type='" + type + "' seg-num=" + s + "><a " + style
                + " href='#'>" + type + " SEGMENT " + (parseInt(s) + 1)
                + "</a> <span class='badge'>" + hops + "</span>";
        html += "<ul>";
        html += "<li><a href='#'>Expiration: " + exp.toLocaleDateString() + " "
                + exp.toLocaleTimeString() + "</a>";
        for (i in if_) {
            html += "<li><a href='#'>" + if_[i].ISD + "-" + if_[i].AS + " ("
                    + if_[i].IFID + ")</a>";
        }
        html += "</ul>";
    }
    return html;
}

function get_json_seg_topo(paths, segs, src, dst) {
    var now = Date.now();
    var outSegs = {
        "core_segments" : {
            "if_lists" : []
        },
        "up_segments" : {
            "if_lists" : []
        },
        "down_segments" : {
            "if_lists" : []
        }
    };
    if (typeof segs == 'undefined') {
        return outSegs;
    }
    var segments = segs; // edit local copy
    console.debug("segments received:", segments.length);
    // loop through all paths as benchmark for filtering segments
    var pathIAs = [];
    var pathStr = "";
    for (p in paths) {
        pathStr += "| ";
        var if_ = paths[p].Entry.Path.Interfaces;
        for (i in if_) {
            var ia = iaRaw2Read(if_[i].RawIsdas);
            if (!pathIAs.includes(ia)) {
                pathIAs.push(ia);
            }
            pathStr += ia + " " + if_[i].IfID + " ";
        }
    }
    // segments must be removed where they do not follow the paths
    var coreIngr = [];
    var coreEgr = [];
    var segOrders = [];
    for (var s = segments.length - 1; s >= 0; s--) {
        var segmentIAs = [];
        var segStr = "", revStr = "";
        var if_ = segments[s].Interfaces;
        var extraIA = false;
        for (i in if_) {
            if (!segmentIAs.includes(if_[i].IA)) {
                segmentIAs.push(if_[i].IA);
            }
            if (!pathIAs.includes(if_[i].IA)) {
                extraIA = true;
            }
            segStr += if_[i].IA + " " + if_[i].IfNum + " ";
            revStr += if_[if_.length - 1 - i].IA + " "
                    + if_[if_.length - 1 - i].IfNum + " ";
        }
        // TODO: (mwfarb) this filtering logic should eventually move to Go
        var exp = new Date(segments[s].Expiry).getTime();
        if (exp < now) {
            // segment IAs must not be expired
            segments.splice(s, 1);
        } else if (extraIA) {
            // segment IAs must appear in at least one path
            segments.splice(s, 1);
        } else if (!(pathStr.includes(segStr) || pathStr.includes(revStr))) {
            // sub-segments must be interrogated for path match, any order
            segments.splice(s, 1);
        } else if (segOrders.includes(segStr) || segOrders.includes(revStr)) {
            // duplicate segment IAs+IFs, any order
            segments.splice(s, 1);
        } else if (segments[s].SegType == "up") {
            if (!segmentIAs.includes(src)) {
                // up segments must include src
                segments.splice(s, 1);
            } else { // valid core ingress
                coreIngr.push(getUpDownCoreTransit(segmentIAs, src, dst));
            }
        } else if (segments[s].SegType == "down") {
            if (!segmentIAs.includes(dst)) {
                // down segments must include dst
                segments.splice(s, 1);
            } else { // valid core egress
                coreEgr.push(getUpDownCoreTransit(segmentIAs, src, dst));
            }
        } else {
            segOrders.push(segStr);
            segOrders.push(revStr);
        }
    }
    // up(src)/down(dst)
    if (coreIngr.length == 0) {
        coreIngr.push(src);
    }
    if (coreEgr.length == 0) {
        coreEgr.push(dst);
    }
    // loop through all segments again, remove unlogical core seg, final format
    var fwd = true;
    for (var s = segments.length - 1; s >= 0; s--) {
        var if_ = segments[s].Interfaces;
        if (segments[s].SegType == "core") {
            // core IAs should be evaluated for matching to/from
            var cs = segments[s];
            var last = if_.length - 1;
            var head = if_[0].IA;
            var tail = if_[last].IA;
            // core segs must follow ingress/egress links from up/down segs
            if (coreIngr.includes(head) && coreEgr.includes(tail)) {
                console.debug("fwd", segments[s].SegType, head, tail);
            } else if (coreIngr.includes(tail) && coreEgr.includes(head)) {
                console.debug("rev", segments[s].SegType, head, tail);
            } else {
                segments.splice(s, 1);
                continue;
            }
        }
        var interfaces = [];
        for (i in if_) {
            var iface = {};
            var ia = if_[i].IA.split('-');
            iface.IFID = if_[i].IfNum;
            iface.ISD = ia[0];
            iface.AS = ia[1];
            interfaces.push(iface);
        }
        var ifaces = {};
        ifaces.interfaces = interfaces;
        ifaces.expTime = new Date(segments[s].Expiry).getTime() / 1000;
        outSegs[segments[s].SegType + "_segments"].if_lists.push(ifaces);
    }
    console.debug("segments post trim:", segments.length);
    return outSegs;
}

function getUpDownCoreTransit(segmentIAs, src, dst) {
    if (src == segmentIAs[0] || dst == segmentIAs[0]) {
        return segmentIAs[segmentIAs.length - 1];
    } else {
        return segmentIAs[0];
    }
}

function get_json_paths(paths, src, dst) {
    var json_paths = {};
    var if_lists = [];
    for (p in paths) {
        var if_ = paths[p].Entry.Path.Interfaces;
        var ifaces = {};
        var interfaces = [];
        for (i in if_) {
            var iface = {};
            var ia = iaRaw2Read(if_[i].RawIsdas).split('-');
            iface.IFID = if_[i].IfID;
            iface.ISD = ia[0];
            iface.AS = ia[1];
            interfaces.push(iface);
        }
        ifaces.interfaces = interfaces;
        ifaces.expTime = paths[p].Entry.Path.ExpTime;
        if_lists.push(ifaces);
    }
    json_paths.if_lists = if_lists;
    return json_paths;
}

function get_json_path_links(paths, csegs, usegs, dsegs) {
    var nsegs = csegs.length + usegs.length + dsegs.length;
    if (nsegs > 0) {
        nonseg_ltype = "PEER";
    } else {
        nonseg_ltype = "CHILD";
    }
    var hops = [];
    c = get_seg_links(csegs, "CORE");
    u = get_seg_links(usegs, "PARENT");
    d = get_seg_links(dsegs, "PARENT");
    n = get_nonseg_links(paths, nonseg_ltype);
    hops = hops.concat(c, u, d, n);
    return hops;
}

function get_seg_links(segs, lType) {
    var hops = [];
    for (s in segs.if_lists) {
        var if_ = segs.if_lists[s].interfaces;
        for (i in if_) {
            var link = {};
            if (i < (if_.length - 1)) {
                link.a = if_[parseInt(i)].ISD + "-" + if_[parseInt(i)].AS;
                link.b = if_[parseInt(i) + 1].ISD + "-"
                        + if_[parseInt(i) + 1].AS;
                link.ltype = lType;
                // TODO: (mwfarb) confirm either direction is not a duplicate
                hops.push(link);
            }
        }
    }
    return hops;
}

function get_nonseg_links(paths, lType) {
    var hops = [];
    var ias = [];
    for (p in paths.if_lists) {
        var if_ = paths.if_lists[p].interfaces;
        for (i in if_) {
            var link = {};
            if (i < (if_.length - 1)) {
                link.a = if_[parseInt(i)].ISD + "-" + if_[parseInt(i)].AS;
                link.b = if_[parseInt(i) + 1].ISD + "-"
                        + if_[parseInt(i) + 1].AS;
                link.ltype = lType;
                // TODO: (mwfarb) confirm either direction is not a duplicate
                hops.push(link);
            }
        }
    }
    return hops;
}

function requestPaths() {
    // make sure to get path topo after IAs are loaded
    var form_data = $('#command-form').serializeArray();
    $("#as-error").empty();
    $.ajax({
        url : 'getpathtopo',
        type : 'post',
        dataType : "json",
        data : form_data,
        success : function(data, textStatus, jqXHR) {
            console.info(JSON.stringify(data));
            if (data.err) {
                showError(data.err);
            }
            var src = $('#ia_cli').val();
            var dst = $('#ia_ser').val();
            resPath = get_json_paths(data.paths, src, dst);
            resSegs = get_json_seg_topo(data.paths, data.segments, src, dst);
            resCore = resSegs.core_segments;
            resUp = resSegs.up_segments;
            resDown = resSegs.down_segments;

            // store incoming paths
            for (var idx = 0; idx < resPath.if_lists.length; idx++) {
                var hops = formatPathString(resPath, idx, 'PATH');
                if (!jPathColors.includes(hops)) {
                    jPathColors.push(hops);
                }
            }

            jTopo = get_json_path_links(resPath, resCore, resUp, resDown);
            $('#path-info').html(
                    get_path_html(data.paths, resCore, resUp, resDown, true));

            // clear graphs, new paths, remove selection
            removePaths();
            self.segType = undefined;
            self.segNum = undefined;

            // setup path config based on defaults loaded
            setupDebug();
            ajaxConfig();

            // path info label switches
            $('#switch_as_names').change(function() {
                handleAsLabelSwitch();
            });
            $('#switch_as_numbers').change(function() {
                handleAsLabelSwitch();
            });
            // map/topology switch
            $('input[type=radio][name=radioPaths]').change(function() {
                handleMapTopologySwitch(false);
            });
        },
        error : function(jqXHR, textStatus, errorThrown) {
            showError(this.url + ' ' + textStatus + ': ' + errorThrown);
        },
    });
}

function loadPathData(src, dst) {
    // ensure local hops data flows deterministically
    orderPaths(src, dst);

    setupPathSelection();

    // update path interfaces with a note when geocode missing
    highlightNoGeoCode(src);

    // setup tree now that we've modified it
    setupListTree();

    // load path topology
    handleMapTopologySwitch(true);
}
