var resSegs, resCore, resUp, resDown, resPath, jTopo, json_as_topo;
var iaLabels;
var iaLocations = [];
var iaGeoLoc;
var g = {};

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

function get_path_html(paths, csegs, usegs, dsegs, show_segs) {
    var html = "<ul class='tree'>";
    for (p in paths) {
        html += "<li seg-type='PATH' seg-num=" + p + "><a href='#'>PATH "
                + (parseInt(p) + 1) + "</a>";
        var ent = paths[p].Entry;
        var exp = new Date(0);
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
        if_ = ent.Path.Interfaces;
        var hops = if_.length / 2;
        html += "<li><a href='#'>Hops: " + hops + "</a>";
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

function get_segment_info(segs, type) {
    var html = "";
    for (s in segs.if_lists) {
        html += "<li seg-type='" + type + "' seg-num=" + s + "><a href='#'>"
                + type + " SEGMENT " + (parseInt(s) + 1) + "</a>";
        var exp = new Date(0);
        exp.setUTCSeconds(segs.if_lists[s].expTime);
        html += "<ul>";
        html += "<li><a href='#'>Expiration: " + exp.toLocaleDateString() + " "
                + exp.toLocaleTimeString() + "</a>";
        if_ = segs.if_lists[s].interfaces;
        var hops = if_.length / 2;
        html += "<li><a href='#'>Hops: " + hops + "</a>";
        for (i in if_) {
            html += "<li><a href='#'>" + if_[i].ISD + "-" + if_[i].AS + " ("
                    + if_[i].IFID + ")</a>";
        }
        html += "</ul>";
    }
    return html;
}

function get_json_seg_topo(paths, segments, src, dst) {
    var now = Date.now();
    var segs = {
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
    for (s in segments) {
        var segmentIAs = [];
        var segStr = "", revStr = "";
        var if_ = segments[s].Interfaces;
        var ifaces = {};
        var interfaces = [];
        var extraIA = false;
        for (i in if_) {
            if (!segmentIAs.includes(if_[i].IA)) {
                segmentIAs.push(if_[i].IA);
            }

            if (!pathIAs.includes(if_[i].IA)) {
                extraIA = true;
            }
            var iface = {};
            var ia = if_[i].IA.split('-');
            iface.IFID = if_[i].IfNum;
            iface.ISD = ia[0];
            iface.AS = ia[1];
            interfaces.push(iface);
            segStr += if_[i].IA + " " + if_[i].IfNum + " ";
            revStr += if_[if_.length - 1 - i].IA + " "
                    + if_[if_.length - 1 - i].IfNum + " ";
        }
        // filter segments not included in paths
        var exp = new Date(segments[s].Expiry).getTime();
        if (exp < now) {
            // segment IAs must not be expired
            console.error("E", exp - now, segments[s].SegType, segmentIAs)
            continue;
        } else if (extraIA) {
            // segment IAs must appear in at least one path
            console.error("I", exp - now, segments[s].SegType, segmentIAs)
            continue;
        }
        // TODO: core IAs should be evaluated for matching to/from
        // up(src)/down(dst)
        // else if (segments[s].SegType == "core"
        // && !(pathStr.includes(segStr) /* || pathStr.includes(revStr) */)) {
        // // core ia+interface segments must appear in at least one path
        // console.error("C", exp - now, segments[s].SegType, segmentIAs)
        // continue;
        // }
        else if (segments[s].SegType == "up" && !segmentIAs.includes(src)) {
            // up segments must include src
            console.error("S", exp - now, segments[s].SegType, segmentIAs)
            continue;
        } else if (segments[s].SegType == "down" && !segmentIAs.includes(dst)) {
            // down segments must include dst
            console.error("D", exp - now, segments[s].SegType, segmentIAs)
            continue;
        } else {
            console.debug("   ", exp - now, segments[s].SegType, segmentIAs)
        }
        ifaces.interfaces = interfaces;
        ifaces.expTime = new Date(segments[s].Expiry).getTime() / 1000;
        segs[segments[s].SegType + "_segments"].if_lists.push(ifaces);
    }
    return segs;
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
