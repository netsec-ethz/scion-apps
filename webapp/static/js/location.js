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

var C_MAP_COUNDEF = '#C0E0BB';
var C_MAP_COUN = '#3366CC';
var C_MAP_COUN_SEL = '#CCCCCC';
var C_MAP_PATH_TOPO = '#999999';
var C_MAP_PATH_ACTIVE = '#FF0000';
var C_MAP_ISD_BRD = '#FFFFFF';

var C_MAP_ISDS = [ '#0099FF', '#FF9900', '#FF0099', '#9900FF', '#00FF99',
        '#99FF00' ];

function sortFloat(a, b) {
    return a - b;
}

function updateMapAsLinks(res, path) {
    if (d_map) {
        updateDMapAsLinks(res, path);
    } else if (wv_map) {
        updateGMapAsLinks(res, path);
    }
}

function updateMapAsMarkers(src, dst) {
    if (d_map) {
        updateDMapAsMarkers(src, dst);
    } else if (wv_map) {
        updateGMapAsMarkers(src, dst);
    }
}

function updateMapIsdRegions(isds) {
    if (d_map) {
        updateDMapIsdRegions(isds);
    } else if (wv_map) {
        updateGMapIsdRegions(isds);
    }
}

/**
 * Return the coordinates of all ASes.
 */
function getAllGeocoordinates() {
    var loc = [];
    var isdAs;
    for (isdAs in self.jLoc) {
        var isdAs = self.jLoc[key];
        var coord = [ isdAs.lat, isdAs.lng ];
        loc.push(coord);
    }
    console.log('all coordinates', loc);
    return loc;
}

/**
 * Calculate bounded center of a set of coordinates.
 */
function getLatLngBoundedCenter(latLngInDegr) {
    var lats = [];
    var lngs = [];
    for (var i = 0; i < latLngInDegr.length; i++) {
        lats.push(parseFloat(latLngInDegr[i][0]));
        lngs.push(parseFloat(latLngInDegr[i][1]));
    }
    lats.sort(sortFloat);
    lngs.sort(sortFloat);

    // calc TB diff, and BT diff, find center
    var latTop = lats[latLngInDegr.length - 1];
    var latBot = lats[0];
    var b2t = Math.abs(latTop - latBot);
    var lat = latBot + (b2t / 2);

    // calc LR diff, and RL diff, find center
    var lngRgt = lngs[latLngInDegr.length - 1];
    var lngLft = lngs[0];
    var l2r = Math.abs(lngRgt - lngLft);
    var lng = lngLft + (l2r / 2);

    // lat is flush in window, so add 1/3 margin
    var latScale = 180 / b2t * 100 * 0.67;
    // long is already short in window, do not add margin
    var lngScale = 360 / l2r * 100;

    // scale, use least scale to show most map
    var scale = latScale < lngScale ? latScale : lngScale;

    return [ lat, lng, scale ];
}

/**
 * Find all possible routes from topology, returns map coordinates.
 */
function getTopologyLinksAll() {
    // first
    var arcs = [];
    for (var p = 0; p < self.jTopo.length; p++) {
        // we want all ISD-ASes from each A to B link possible
        var aLocs = $.grep(self.jLoc, function(e, i) {
            return e.ia === self.jTopo[p].a;
        });
        var bLocs = $.grep(self.jLoc, function(e, i) {
            return e.ia === self.jTopo[p].b;
        });
        var isdAsA = [ aLocs[0].lat, aLocs[0].lng ];
        var isdAsB = [ bLocs[0].lat, bLocs[0].lng ];
        if (JSON.stringify(isdAsA) == JSON.stringify(isdAsB)) {
            // skip internal routing when making arcs
            continue;
        }
        arcs.push(createLink(isdAsA, isdAsB, C_MAP_PATH_TOPO));
    }
    return arcs;
}

/**
 * Find only the currently selected path, returns map coordinates..
 */
function getPathSelectedLinks(res, path, color) {
    var arcs = [];
    var routes = [];
    if (path < 0) {
        for (var i = 0; i < res.if_lists.length; i++) {
            routes.push(i);
        }
    } else {
        routes.push(path);
    }
    for (var p = 0; p < routes.length; p++) {
        var pNum = parseInt(routes[p]);
        var geol = [];
        for (var ifNum = 0; ifNum < (res.if_lists[pNum].interfaces.length - 1); ifNum++) {
            // determine which endpoints have geolocation
            var iface = res.if_lists[pNum].interfaces[ifNum];
            var ifaceNext = res.if_lists[pNum].interfaces[ifNum + 1];
            var aIf = (iface.ISD + '-' + iface.AS);
            var bIf = (ifaceNext.ISD + '-' + ifaceNext.AS);
            var aLocs = $.grep(self.jLoc, function(e, i) {
                return e.ia === aIf;
            });
            var bLocs = $.grep(self.jLoc, function(e, i) {
                return e.ia === bIf;
            });
            if (aLocs.length > 0) {
                if (JSON.stringify(geol[geol.length - 1]) != JSON
                        .stringify(aLocs[0])) {
                    geol.push(aLocs[0]);
                }
            }
            if (bLocs.length > 0) {
                if (JSON.stringify(geol[geol.length - 1]) != JSON
                        .stringify(bLocs[0])) {
                    geol.push(bLocs[0]);
                }
            }
        }
        for (var x = 0; x < (geol.length - 1); x++) {
            // only link endpoints that have geolocation
            var loc = geol[x];
            var locNext = geol[x + 1];
            var isdAsA = [ loc.lat, loc.lng ];
            var isdAsB = [ locNext.lat, locNext.lng ];
            arcs.push(createLink(isdAsA, isdAsB, color));
        }
    }
    return arcs;
}

/**
 * Formats path link data used for both Datamaps and Webcola map UI.
 *
 * @param isdAsA
 *            Link start "ISD-AS".
 * @param isdAsB
 *            Link end "ISD-AS".
 * @param color
 *            RGB color of link path.
 * @returns Link object used to draw.
 */
function createLink(isdAsA, isdAsB, color) {
    return {
        origin : {
            latitude : isdAsA[0],
            longitude : isdAsA[1]
        },
        destination : {
            latitude : isdAsB[0],
            longitude : isdAsB[1]
        },
        options : {
            strokeColor : color
        }
    };
}

/**
 * Find marker locations of all ASes with special notes for source/destination.
 */
function getMarkerLocations(src, dst) {
    var loc = [];
    for (key in self.jLoc) {
        var isdAs = self.jLoc[key];
        var ifNum = isdAs.ia.split('-');
        var marker = '', rad = 4;
        if (src != null && isdAs.ia == src) {
            marker = 'S';
            rad = 8;
        } else if (dst != null && isdAs.ia == dst) {
            marker = 'D';
            rad = 8;
        }
        var bubble = {
            ia : isdAs.ia,
            name : isdAs.host,
            marker : marker,
            latitude : isdAs.lat,
            longitude : isdAs.lng,
            radius : rad,
            fillKey : "ISD-" + ifNum[0],
        };
        loc.push(bubble);
    }
    return loc;
}
