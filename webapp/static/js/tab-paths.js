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
        $("#as-pathtopo").children("svg").remove();
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
            if (topologyUpdate) {
                drawMap(g.src, g.dst, iaGeoLoc);
            }
            handleAsLabelSwitch();
        }
    } else {
        $("#as-pathtopo").children("iframe").hide();
        // only load svg once
        if ($("#as-pathtopo").children("svg").length == 0) {
            // load path topology
            var width = $("#as-pathtopo").width();
            var height = $("#as-pathtopo").height();
            drawTopology("as-pathtopo", jTopo, resSegs, width, height);
            // add endpoint labels
            var open = typeof self.segType !== 'undefined';
            setPaths(self.segType, self.segNum, open);
            topoSetup({
                "source" : g.src,
                "destination" : g.dst,
            }, width, height);
        } else {
            $("#as-pathtopo").children("svg").show();
        }
        handleAsLabelSwitch();
    }
}

function get_path_info(paths) {
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
    html += "</ul>";
    return html;
}

// TODO: need segments to construct segments topology
function get_json_seg_topo(paths) {
    return {
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
}

function get_json_paths(paths) {
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

// TODO: need segments to add "PEER" ltype
function get_json_path_topo(paths) {
    var hops = [];
    for (p in paths) {
        var if_ = paths[p].Entry.Path.Interfaces;
        for (i in if_) {
            var link = {};
            if (i < (if_.length - 1)) {
                curIa = if_[parseInt(i)].RawIsdas;
                nextIa = if_[parseInt(i) + 1].RawIsdas;
                link.a = iaRaw2Read(curIa);
                link.b = iaRaw2Read(nextIa);
                link.ltype = "PARENT";
                hops.push(link);
            }
        }
    }
    return hops;
}

function requestPaths() {
    // make sure to get path topo after IAs are loaded
    var form_data = $('#command-form').serializeArray();
    $.ajax({
        url : 'getpathtopo',
        type : 'post',
        dataType : "json",
        data : form_data,
        success : function(data, textStatus, jqXHR) {
            console.info(JSON.stringify(data));
            showError(data.err);

            resSegs = get_json_seg_topo(data.paths);
            resCore = resSegs.core_segments;
            resUp = resSegs.up_segments;
            resDown = resSegs.down_segments;
            resPath = get_json_paths(data.paths);
            jTopo = get_json_path_topo(data.paths);
            $('#path-info').html(get_path_info(data.paths));

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
