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

// https://github.com/jquery/jquery
// https://github.com/d3/d3
// https://github.com/twbs/bootstrap
// https://github.com/aterrien/jquery-knob
// https://github.com/lokesh-coder/pretty-checkbox
// https://github.com/iconic/open-iconic
// https://code.highcharts.com

var commandProg;
var intervalGraphTick;
var intervalGraphData;
var secMax = 10;
var secMin = 1;
var sizeMax = 1400; // TODO: pull from MTU
var sizeMin = 64;
var pktMax = 187500000;
var pktMin = 1;
var bwMax = 150.00;
var bwMin = 0.0000001;
var feedback = {};
var nodes = {};

var granularity = 5;
var xAxisSec = 60;
var ticks = xAxisSec * granularity;
var tickMs = 1000 / granularity;
var xLeftTrimMs = 1000 / granularity;
var bwIntervalBufMs = 1000;
var dataIntervalMs = 1000;
var progIntervalMs = 500
var chartCS;
var chartSC;
var chartSE;
var lastTime;
var lastTimeBwDb = new Date((new Date()).getTime() - (xAxisSec * 1000));

var dial_prop_all = {
    // dial constants
    'width' : '100',
    'height' : '100',
    "angleOffset" : "-125",
    "angleArc" : "250",
    "inputColor" : "#000",
    "lineCap" : "default",
};
var dial_prop_arc = {
    'cursor' : true,
    'fgColor' : '#f00',
    'bgColor' : '#fff',
};
var dial_prop_text = {
    'fgColor' : '#0000', // opaque
    'bgColor' : '#0000', // opaque
};

// instruction information
var bwText = 'Dial values can be typed, edited, clicked, or scrolled to change.';
var imageText = 'Execute camerapp to retrieve an image.';
var sensorText = 'Execute sensorapp to retrieve sensor data.';
var bwgraphsText = 'Click legend to hide/show data when continuous test is on.';
var cont_disable_msg = 'Continuous testing disabled.'
var echoText = 'Execute echo to measure response time.';

window.onbeforeunload = function(event) {
    // detect window close to end continuous test if any
    command(false);
};

function failContinuousOff() {
    var checked = $('#switch_cont').prop('checked');
    if (!checked) {
        // send command to end continuous test
        command(false);
    }
}

function initBwGraphs() {
    // continuous test default: off
    $('#switch_cont').prop("checked", false);
    $('#bwtest-graphs').css("display", "block");
    // continuous test switch
    $('#switch_cont').change(function() {
        var checked = $(this).prop('checked');
        handleSwitchContTest(checked);
    });

    var checked = $('#switch_utc').prop('checked');
    setChartUtc(checked);
    $('#switch_utc').change(function() {
        var checked = $(this).prop('checked');
        setChartUtc(checked);
    });

    updateBwInterval();

    // charts update on tab switch
    $('a[data-toggle="tab"]').on('shown.bs.tab', function(e) {
        var name = $(e.target).attr("name");
        if (name != "as-graphs" && name != "as-tab-pathtopo") {
            handleSwitchTabs();
        }
    });
    // setup charts
    var csColAch = $('#svg-client circle').css("fill");
    var scColAch = $('#svg-server circle').css("fill");
    var csColReq = $('#svg-cs line').css("stroke");
    var scColReq = $('#svg-sc line').css("stroke");
    chartCS = drawBwtestSingleDir('cs', 'Upload (mbps)', false, csColReq,
            csColAch);
    chartSC = drawBwtestSingleDir('sc', 'Download (mbps)', true, scColReq,
            scColAch);
    chartSE = drawPingGraph('echo-graph', 'Echo Response (ms)');
    // setup interval to manage smooth ticking
    lastTime = (new Date()).getTime() - (ticks * tickMs) + xLeftTrimMs;
    manageTickData();
    manageTestData();
}

function showOnlyConsoleGraphs(activeApp) {
    $('#bwtest-continuous').css("display",
            (activeApp == "bwtester") ? "block" : "none");
    $('#images').css("display", (activeApp == "camerapp") ? "block" : "none");
    $('#echo-continuous').css("display",
            (activeApp == "echo") ? "block" : "none");
    var isConsole = (activeApp == "bwtester" || activeApp == "camerapp"
            || activeApp == "sensorapp" || activeApp == "echo" || activeApp == "traceroute");
    $('.stdout').css("display", isConsole ? "block" : "none");
}

function handleSwitchTabs() {
    var activeApp = $('.nav-tabs .active > a').attr('name');
    var isCont = (activeApp == "bwtester" || activeApp == "echo" || activeApp == "traceroute");
    enableContControls(isCont);
    // show/hide graphs for bwtester
    showOnlyConsoleGraphs(activeApp);
    var checked = $('#switch_cont').prop('checked');
    if (checked && !isCont) {
        $("#switch_cont").prop('checked', false);
        enableTestControls(true);
        releaseTabs();
        show_temp_err(cont_disable_msg);
    }
}

function handleSwitchContTest(checked) {
    if (checked) {
        enableTestControls(false);
        var activeApp = $('.nav-tabs .active > a').attr('name');
        lockTab(activeApp);
        // starts continuous tests
        manageTestData();
    } else {
        // end continuous tests
        command(false);
        enableTestControls(true);
        releaseTabs();
        clearInterval(intervalGraphData);
    }
}

function setChartUtc(useUTC) {
    Highcharts.setOptions({
        global : {
            useUTC : useUTC
        }
    });
}

function getBwParamDisplay() {
    return getBwParamLine('cs') + ' / ' + getBwParamLine('sc');
}

function getBwParamLine(dir) {
    return dir + ': ' + $('#dial-' + dir + '-sec').val() + 's, '
            + $('#dial-' + dir + '-size').val() + 'b x '
            + $('#dial-' + dir + '-pkt').val() + ' pkts, '
            + $('#dial-' + dir + '-bw').val() + ' Mbps';
}

function drawBwtestSingleDir(dir, yAxisLabel, legend, reqCol, achCol) {
    var div_id = dir + "-bwtest-graph";
    var chart = Highcharts.chart(div_id, {
        chart : {
            type : 'scatter',
            animation : Highcharts.svg,
            marginRight : 10,
        },
        title : {
            text : null
        },
        xAxis : {
            type : 'datetime',
            tickPixelInterval : 150,
            crosshair : true,
        },
        yAxis : [ {
            title : {
                text : yAxisLabel
            },
            gridLineWidth : 1,
            min : 0,
        } ],
        tooltip : {
            enabled : true,
            formatter : formatBwTooltip,
        },
        legend : {
            y : -15,
            layout : 'horizontal',
            align : 'right',
            verticalAlign : 'top',
            floating : true,
            enabled : true,
        },
        credits : {
            enabled : legend,
            text : legend ? 'Download Data' : null,
            href : legend ? './data/' : null,
        },
        exporting : {
            enabled : false
        },
        plotOptions : {},
        series : [ {
            name : 'attempted',
            data : loadSetupData(),
            marker : {
                symbol : 'triangle-down'
            },
        }, {
            name : 'achieved',
            data : loadSetupData(),
            marker : {
                symbol : 'triangle'
            },
            dataLabels : {
                enabled : true,
                formatter : function() {
                    return Highcharts.numberFormat(this.y, 2)
                },
            },
        } ]
    });
    return chart;
}

function drawPingGraph(div_id, yAxisLabel) {
    var chart = Highcharts.chart(div_id, {
        chart : {
            type : 'column'
        },
        title : {
            text : null
        },
        xAxis : {
            type : 'datetime',
        },
        yAxis : {
            title : {
                text : yAxisLabel
            }
        },
        legend : {
            enabled : false
        },
        tooltip : {
            enabled : true,
            formatter : formatPingTooltip,
        },
        credits : {
            enabled : true,
            text : 'Download Data',
            href : './data/',
        },
        exporting : {
            enabled : false
        },
        plotOptions : {
            column : {
                pointWidth : 8
            }
        },
        series : [ {
            name : yAxisLabel,
            data : loadSetupData(),
            dataLabels : {
                enabled : false,
            }
        } ]
    });
    return chart;
}

function formatBwTooltip() {
    var tooltip = '<b>' + this.series.name + '</b><br/>';
    tooltip += Highcharts.dateFormat('%Y-%m-%d %H:%M:%S', this.x) + '<br/>';
    tooltip += Highcharts.numberFormat(this.y, 2) + ' mbps<br/>';
    tooltip += '<i>' + this.point.path + '</i>';
    if (this.point.error != null) {
        tooltip += '<br/><b>' + this.point.error + '</b>';
    }
    return tooltip;
}

function formatPingTooltip() {
    var tooltip = '<b>' + this.series.name + '</b><br/>';
    tooltip += Highcharts.dateFormat('%Y-%m-%d %H:%M:%S', this.x) + '<br/>';
    tooltip += Highcharts.numberFormat(this.y, 3) + ' ms<br/>';
    if (this.point.loss > 0) {
        tooltip += this.point.loss + '% packet loss<br/>';
    }
    tooltip += '<i>' + this.point.path + '</i>';
    if (this.point.error != null) {
        tooltip += '<br/><b>' + this.point.error + '</b>';
    }
    return tooltip;
}

function loadSetupData() {
    // points are a function of timeline speed (width & seconds)
    // no data points on setup
    var data = [], time = (new Date()).getTime(), i;
    for (i = -ticks; i <= 0; i += 1) {
        data.push({
            x : time + i * tickMs,
            y : null
        });
    }
    return data;
}

function manageTickData() {
    // add placeholders for time ticks
    ticks = xAxisSec * granularity;
    tickMs = 1000 / granularity;
    xLeftTrimMs = 1000 / granularity;
    clearInterval(intervalGraphTick); // prevent overlap
    intervalGraphTick = setInterval(function() {
        var newTime = (new Date()).getTime();
        refreshTickData(chartCS, newTime);
        refreshTickData(chartSC, newTime);
        refreshTickData(chartSE, newTime);
    }, tickMs);
}

function manageTestData() {
    // setup interval to request data point updates, only in range
    var now = (new Date()).getTime();
    maxTimeBwDb = (new Date(now - (xAxisSec * 1000))).getTime();
    lastTimeBwDb = (lastTimeBwDb < maxTimeBwDb ? maxTimeBwDb : lastTimeBwDb);
    clearInterval(intervalGraphData); // prevent overlap
    intervalGraphData = setInterval(function() {
        // update continuous test parameters
        var checked = $('#switch_cont').prop('checked');
        if (checked) {
            command(true);
        }
        now = (new Date()).getTime();
        // update continuous results
        var form_data = {
            since : lastTimeBwDb
        };
        var activeApp = $('.nav-tabs .active > a').attr('name');
        if (!commandProg && checked) {
            handleStartCmdDisplay(activeApp);
        }
        console.info('req:', JSON.stringify(form_data));
        if (activeApp == "bwtester") {
            requestBwTestByTime(form_data);
        } else if (activeApp == "echo") {
            requestEchoByTime(form_data);
        } else if (activeApp == "traceroute") {
            requestTraceRouteByTime(form_data);
        }
        lastTimeBwDb = now;
    }, dataIntervalMs);
}

function requestBwTestByTime(form_data) {
    $.post("/getbwbytime", form_data, function(json) {
        var d = JSON.parse(json);
        console.info('resp:', JSON.stringify(d));
        if (d != null) {
            if (d.active != null) {
                if (d.active) {
                    enableTestControls(false);
                    lockTab("bwtester");
                    failContinuousOff();
                } else {
                    enableTestControls(true);
                    releaseTabs();
                    clearInterval(intervalGraphData);
                }
            }
            if (d.graph != null) {
                // write data on graph
                for (var i = 0; i < d.graph.length; i++) {
                    if (d.graph[i].Log != null && d.graph[i].Log != "") {
                        // result returned, display it and reset progress
                        handleEndCmdDisplay(d.graph[i].Log);
                    }
                    var data = {
                        'cs' : {
                            'bandwidth' : d.graph[i].CSBandwidth,
                            'throughput' : d.graph[i].CSThroughput,
                            'path' : d.graph[i].Path,
                        },
                        'sc' : {
                            'bandwidth' : d.graph[i].SCBandwidth,
                            'throughput' : d.graph[i].SCThroughput,
                            'path' : d.graph[i].Path,
                        },
                    };
                    // update with errors, if any
                    updateBwErrors(data.cs, 'cs', d.graph[i].Error);
                    updateBwErrors(data.sc, 'sc', d.graph[i].Error);

                    console.info(JSON.stringify(data));
                    console.info('continuous bwtester', 'duration:',
                            d.graph[i].ActualDuration, 'ms');
                    // use the time the test began
                    var time = d.graph[i].Inserted - d.graph[i].ActualDuration;
                    updateBwGraph(data, time)
                }
            }
        }
    });
}

function requestEchoByTime(form_data) {
    $.post("/getechobytime", form_data, function(json) {
        var d = JSON.parse(json);
        console.info('resp:', JSON.stringify(d));
        if (d != null) {
            if (d.active != null) {
                if (d.active) {
                    enableTestControls(false);
                    lockTab("echo");
                    failContinuousOff();
                } else {
                    enableTestControls(true);
                    releaseTabs();
                    clearInterval(intervalGraphData);
                }
            }
            if (d.graph != null) {
                // write data on graph
                for (var i = 0; i < d.graph.length; i++) {
                    if (d.graph[i].CmdOutput != null
                            && d.graph[i].CmdOutput != "") {
                        // result returned, display it and reset progress
                        handleEndCmdDisplay(d.graph[i].CmdOutput);
                    }
                    var data = {
                        'responseTime' : d.graph[i].ResponseTime,
                        'runTime' : d.graph[i].RunTime,
                        'loss' : d.graph[i].PktLoss,
                        'path' : d.graph[i].Path,
                        'error' : d.graph[i].Error,
                    };
                    if (data.runTime == 0) {
                        // for other errors, use execution time
                        data.runTime = d.graph[i].ActualDuration;
                    }
                    console.info(JSON.stringify(data));
                    console.info('continous echo', 'duration:',
                            d.graph[i].ActualDuration, 'ms');
                    // use the time the test began
                    var time = d.graph[i].Inserted - d.graph[i].ActualDuration;
                    updatePingGraph(chartSE, data, time)
                }
            }
        }
    });
}

function requestTraceRouteByTime(form_data) {
    $.post("/gettraceroutebytime", form_data, function(json) {
        var d = JSON.parse(json);
        console.info('resp:', JSON.stringify(d));
        if (d != null) {
            if (d.active != null) {
                $('#switch_cont').prop("checked", d.active);
                if (d.active) {
                    enableTestControls(false);
                    lockTab("traceroute");
                } else {
                    enableTestControls(true);
                    releaseTabs();
                    clearInterval(intervalGraphData);
                }
            }
            if (d.graph != null) {
                // write data on graph
                for (var i = 0; i < d.graph.length; i++) {
                    if (d.graph[i].CmdOutput != null
                            && d.graph[i].CmdOutput != "") {
                        // result returned, display it and reset progress
                        handleEndCmdDisplay(d.graph[i].CmdOutput);
                    }

                    console.info('continous traceroute', 'duration:',
                            d.graph[i].ActualDuration, 'ms');

                    // TODO (mwfarb): implement traceroute graph
                }
            }
        }
    });
}

function refreshTickData(chart, newTime) {
    var x = newTime, y = null;
    var series0 = chart.series[0];
    var series1 = chart.series[1];
    var shift = false;

    lastTime = x - (ticks * tickMs) + xLeftTrimMs;
    // manually remove all left side ticks < left side time
    // wait for adding hidden ticks to draw
    var draw = false;
    if (series0)
        removeOldPoints(series0, lastTime, draw);
    if (series1)
        removeOldPoints(series1, lastTime, draw);
    // manually add hidden right side ticks, time = now
    // do all drawing here to avoid accordioning redraws
    // do not shift points since we manually remove before this
    draw = true;
    if (series0)
        series0.addPoint([ x, y ], draw, shift);
    if (series1)
        series1.addPoint([ x, y ], draw, shift);
}

function removeOldPoints(series, lastTime, draw) {
    for (var i = 0; i < series.data.length; i++) {
        if (series.data[i] && series.data[i].x < lastTime) {
            series.removePoint(i, draw);
        }
    }
}

function updateBwGraph(data, time) {
    updateBwChart(chartCS, data.cs, time);
    updateBwChart(chartSC, data.sc, time);
}

function updateBwChart(chart, dataDir, time) {
    var bw = dataDir.bandwidth / 1000000;
    var tp = dataDir.throughput / 1000000;
    var loss = dataDir.throughput / dataDir.bandwidth;
    // manually add visible right side ticks, time = now
    // wait for adding hidden ticks to draw, for consistancy
    // do not shift points since we manually remove before this
    var draw = false;
    var shift = false;
    var color = getPathColor(dataDir.path.match("\\[.*]"));
    if (dataDir.error) {
        chart.series[0].addPoint({
            x : time,
            y : bw,
            path : dataDir.path,
            error : dataDir.error,
            color : '#ff000033', // errors in faint red
            marker : {
                symbol : 'diamond',
            }
        }, draw, shift);
    } else {
        chart.series[0].addPoint({
            x : time,
            y : bw,
            path : dataDir.path,
            color : color,
        }, draw, shift);
    }
    if (tp > 0) {
        chart.series[1].addPoint({
            x : time,
            y : tp,
            path : dataDir.path,
            color : color,
        }, draw, shift);
    }
}

function updatePingGraph(chart, data, time) {
    // manually add visible right side ticks, time = now
    // wait for adding hidden ticks to draw, for consistancy
    // do not shift points since we manually remove before this
    var draw = false;
    var shift = false;
    var color = getPathColor(data.path.match("\\[.*]"))
    if (data.error || data.loss > 0 || data.responseTime <= 0) {
        var error = 'Command terminated.';
        if (data.loss > 0) {
            error = 'Response timeout.';
        }
        chart.series[0].addPoint({
            x : time,
            y : data.runTime, // errors show full run time
            loss : data.loss,
            path : data.path,
            error : data.error ? data.error : error,
            color : '#ff000033', // errors in faint red
        }, draw, shift);
    } else {
        chart.series[0].addPoint({
            x : time,
            y : data.responseTime,
            loss : data.loss,
            path : data.path,
            color : color,
        }, draw, shift);
    }
}

function endProgress() {
    clearInterval(commandProg);
    commandProg = false;
}

function command(continuous) {
    var startTime = (new Date()).getTime();
    var activeApp = $('.nav-tabs .active > a').attr('name');
    enableTestControls(false);
    lockTab(activeApp);

    // add required client/server address options
    var form_data = $('#command-form').serializeArray();
    form_data.push({
        name : "apps",
        value : activeApp
    });
    if (activeApp == "bwtester" || activeApp == "echo"
            || activeApp == "traceroute") {
        // add extra bwtester options required
        form_data.push({
            name : "continuous",
            value : continuous
        });
        if (self.segType == 'PATH') { // only full paths allowed
            form_data.push({
                name : "pathStr",
                value : formatPathString(resPath, self.segNum, self.segType)
            });
        }
    }
    if (activeApp == "bwtester") {
        form_data.push({
            name : "interval",
            value : getIntervalMax()
        });
    }
    if (activeApp == "echo") {
        form_data.push({
            name : "interval",
            value : $('#echo_sec').val()
        });
    }
    if (activeApp == "camerapp") {
        // clear for new image request
        $('#images').empty();
        $('#image_text').text(imageText);
    }
    if (!continuous) {
        $("#results").empty();
        handleStartCmdDisplay(activeApp);
    }
    console.info('req:', JSON.stringify(form_data));
    $.post('/command', form_data, function(resp, status, jqXHR) {
        console.info('resp:', resp);
        if (!continuous) {
            // continuous flag should force switch
            var duration = (new Date()).getTime() - startTime;
            console.info(activeApp, 'duration:', duration, 'ms');
            handleEndCmdDisplay(resp);
        }
        if (activeApp == "camerapp") {
            // check for new images once, on command complete
            handleImageResponse(resp);
        } else if (activeApp == "bwtester") {
            // check for usable data for graphing
            handleContResponse(resp, continuous, startTime);
        } else if (activeApp == "echo") {
            // check for usable data for graphing
            handleContResponse(resp, continuous, startTime);
        } else if (activeApp == "traceroute") {
            handleContResponse(resp, continuous, startTime);
        } else {
            handleGeneralResponse();
        }
    }).fail(function(error) {
        showError(error.responseJSON);
        handleGeneralResponse();
    });
    // onsubmit should always return false to override native http call
    return false;
}

function handleStartCmdDisplay(activeApp) {
    var i = 1;
    // suspend any pending commands
    if (commandProg) {
        endProgress();
    }
    // $("#results").empty();
    $("#results").append("Executing ");
    $('#results').append(activeApp);
    $('#results').append(" client");
    clearInterval(commandProg); // prevent overlap
    commandProg = setInterval(function() {
        $('#results').append('.');
        i += 1;
    }, progIntervalMs);
}

function handleEndCmdDisplay(resp) {
    $('#results').html(resp);
    $(".stdout").scrollTop($(".stdout")[0].scrollHeight);
    endProgress();
}

function enableTestControls(enable) {
    $("#button_cmd").prop('disabled', !enable);
}

function enableContControls(enable) {
    $("#switch_cont").prop('disabled', !enable);
}

function lockTab(href) {
    enableTab("bwtester", "bwtester" == href);
    enableTab("camerapp", "camerapp" == href);
    enableTab("sensorapp", "sensorapp" == href);
    enableTab("echo", "echo" == href);
    enableTab("traceroute", "traceroute" == href);
}

function releaseTabs() {
    enableTab("bwtester", true);
    enableTab("camerapp", true);
    enableTab("sensorapp", true);
    enableTab("echo", true);
    enableTab("traceroute", true);
}

function enableTab(href, enable) {
    if (enable) {
        $('.nav-tabs a[href="#' + href + '"]').attr("data-toggle", "tab");
        $('.nav-tabs a[href="#' + href + '"]').parent('li').removeClass(
                'disabled');
    } else {
        $('.nav-tabs a[href="#' + href + '"]').removeAttr('data-toggle');
        $('.nav-tabs a[href="#' + href + '"]').parent('li')
                .addClass('disabled');
    }
}

function handleGeneralResponse() {
    enableTestControls(true);
    releaseTabs();
}

function handleImageResponse(resp) {
    if (resp.includes('Done, exiting')) {
        $('#image_text').load('/txtlast');
        $('#images').load('/imglast');
    }
    enableTestControls(true);
    releaseTabs();
}

function getIntervalMax() {
    var cs = $('#dial-cs-sec').val();
    var sc = $('#dial-sc-sec').val();
    var cont = $('#bwtest_sec').val();
    var max = Math.max(cs, sc, cont);
    return max;
}

function handleContResponse(resp, continuous, startTime) {
    // check for continuous testing
    var checked = $('#switch_cont').prop('checked');
    if (!checked && !commandProg) {
        enableTestControls(true);
        releaseTabs();
        clearInterval(intervalGraphData);
    }
    if (!continuous) {
        manageTestData();
    }
}

function updateBwInterval() {
    var cs = $('#dial-cs-sec').val() * 1000;
    var sc = $('#dial-sc-sec').val() * 1000;
    var cont = $('#bwtest_sec').val() * 1000;
    var max = Math.max(cs, sc);
    if (cont != (max + bwIntervalBufMs)) {
        $('#bwtest_sec').val((max + bwIntervalBufMs) / 1000);
    }
    // update interval minimum
    var min = Math.min(cs, sc);
    $('#bwtest_sec').prop('min', min / 1000);
}

function updateBwErrors(dataDir, dir, err) {
    if (!dataDir.throughput || dataDir.throughput == 0) {
        dataDir.error = err;
    }
}

function initNodes() {
    loadNodes('cli', 'clients_default');
    $("a[data-toggle='tab']").on('shown.bs.tab', function(e) {
        var name = $(e.target).attr("name");
        if (name != "as-graphs" && name != "as-tab-pathtopo") {
            updateNodeOptions('ser');
        }
    });
    $('#sel_cli').change(function() {
        updateNode('cli');
        // after client selection, update server options
        loadServerNodes();
    });
    $('#sel_ser').change(function() {
        updateNode('ser');
    });
}

function loadServerNodes() {
    // client 'lo' localhost interface selected, use localhost servers
    var name = $('#sel_cli').find("option:selected").html();
    if (name == "lo") {
        loadNodes('ser', "servers_user");
    } else {
        loadNodes('ser', "servers_default");
    }
}

function loadNodes(node, list) {
    var data = [ {
        name : "node_type",
        value : list
    } ];
    console.info(JSON.stringify(data));
    $('#sel_' + node).load('/getnodes', data, function(resp, status, jqXHR) {
        console.info('resp:', resp);
        if (status == "success") {
            nodes[node] = JSON.parse(resp);
            updateNodeOptions(node);
            if (node == 'cli') {
                // after client selection, update server options
                loadServerNodes();
            }
        } else {
            console.error("Error: " + jqXHR.status + ": " + jqXHR.statusText);
        }
    });
}

function updateNodeOptions(node) {
    var allNode = nodes[node]['all'];
    var activeApp = (allNode != null) ? 'all' : $('.nav-tabs .active > a')
            .attr('name');
    console.debug(activeApp);
    var app_nodes = nodes[node][activeApp];
    $('#sel_' + node).empty();
    for (var i = 0; i < app_nodes.length; i++) {
        $('#sel_' + node).append($('<option>', {
            value : i,
            text : app_nodes[i].name
        }));
    }
    updateNode(node);
}

function updateNode(node) {
    // populate fields
    if (nodes[node]) {
        var allNode = nodes[node]['all'];
        var activeApp = (allNode != null) ? 'all' : $('.nav-tabs .active > a')
                .attr('name');
        var app_nodes = nodes[node][activeApp];
        var sel = $('#sel_' + node).find("option:selected").attr('value');
        if (sel != null) {
            $('#ia_' + node).val(app_nodes[sel].isdas.replace(/_/g, ":"));
            $('#addr_' + node).val(app_nodes[sel].addr);
            $('#port_' + node).val(app_nodes[sel].port);
            if (node == 'ser') {
                // server node change complete, update paths
                requestPaths();
            }
        }
    }
}

function setDefaults() {
    if (commandProg) {
        endProgress();
    }
    $("#results").empty();
    $('#images').empty();
    $('#image_text').text(imageText);
    $('#stats_text').text(sensorText);
    $('#bwtest_text').text(bwText);
    $('#bwgraphs_text').text(bwgraphsText);
    $('#echo_text').text(echoText);

    onchange_radio('cs', 'size');
    onchange_radio('sc', 'size');

    updateNode('cli');
    updateNode('ser');
}

function extend(obj, src) {
    for ( var key in src) {
        if (src.hasOwnProperty(key))
            obj[key] = src[key];
    }
    return obj;
}

function initDials(dir) {
    $('input[type=radio][name=' + dir + '-dial]').on('change', function() {
        onchange_radio(dir, $(this).val());
    });

    var prop_sec = {
        'min' : secMin,
        'max' : secMax,
        'release' : function(v) {
            return onchange(dir, 'sec', v < secMin ? secMin : v);
        },
    };
    var prop_size = {
        'min' : 1, // 1 allows < 64 to be typed
        'max' : sizeMax,
        'release' : function(v) {
            return onchange(dir, 'size', v < 1 ? 1 : v);
        },
    };
    var prop_pkt = {
        'min' : pktMin,
        'max' : pktMax,
        'release' : function(v) {
            return onchange(dir, 'pkt', v < pktMin ? pktMin : v);
        },
        'draw' : function() {
            // allow large font when possible
            var pkt = $('#dial-' + dir + '-pkt').val();
            if (pkt < 999999) {
                $(this.i).css("font-size", "16px");
            } else if (pkt < 9999999) {
                $(this.i).css("font-size", "13px");
            } else {
                $(this.i).css("font-size", "11px");
            }
        },
    };
    var prop_bw = {
        'min' : 0.01, // 0.01 works around library typing issues
        'max' : bwMax,
        'step' : 0.01,
        'release' : function(v) {
            return onchange(dir, 'bw', v < 0.01 ? 0.01 : v);
        },
        'format' : function(v) {
            // native formatting occasionally uses full precision
            // so we format it manually ourselves
            return Number(Math.round(v + 'e' + 2) + 'e-' + 2);
        },
    };
    $('#dial-' + dir + '-sec').knob(extend(prop_sec, dial_prop_arc));
    $('#dial-' + dir + '-size').knob(extend(prop_size, dial_prop_arc));
    $('#dial-' + dir + '-pkt').knob(extend(prop_pkt, dial_prop_text));
    $('#dial-' + dir + '-bw').knob(extend(prop_bw, dial_prop_arc));
}

function onchange_radio(dir, value) {
    console.debug('radio change ' + dir + '-' + value);
    // change read-only status, based on radio change
    switch (value) {
    case 'size':
        setDialLock(dir, 'size', true);
        setDialLock(dir, 'pkt', false);
        setDialLock(dir, 'bw', false);
        break;
    case 'pkt':
        setDialLock(dir, 'size', false);
        setDialLock(dir, 'pkt', true);
        setDialLock(dir, 'bw', false);
        break;
    case 'bw':
        setDialLock(dir, 'size', false);
        setDialLock(dir, 'pkt', false);
        setDialLock(dir, 'bw', true);
        break;
    }
}

function onchange(dir, name, v) {
    // change other dials, when dial values change
    console.debug('? ' + dir + '-' + name + ' ' + 'change' + ':', v);
    if (!feedback[dir + '-' + name]) {
        var lock = $('input[name=' + dir + '-dial]:checked').val();
        switch (name) {
        case 'sec':
            onchange_sec(dir, v, secMin, secMax, lock);
            break;
        case 'size':
            onchange_size(dir, v, sizeMin, sizeMax, lock);
            break;
        case 'pkt':
            onchange_pkt(dir, v, pktMin, pktMax, lock);
            break;
        case 'bw':
            onchange_bw(dir, v, bwMin, bwMax, lock);
            break;
        }
    } else {
        feedback[dir + '-' + name] = false;
    }
}

function update_dial(dir, name, val, min, max) {
    var valid = (val <= max && val >= min);
    if (valid) {
        setTimeout(function() {
            console.debug('> ' + dir + '-' + name + ' trigger blur:', val);
            feedback[dir + '-' + name] = true;
            $('#dial-' + dir + '-' + name).val(val);
            $('#dial-' + dir + '-' + name).trigger('blur')
        }, 1);
    } else {
        console.warn('!> invalid ' + dir + '-' + name + ':', val);
        show_range_err(dir, name, val, min, max);
    }
    return valid;
}

function src_in_range(dir, name, v, min, max) {
    var value = v;
    if (v < min) {
        value = min;
    } else if (v > max) {
        value = max;
    }
    if (value != v) {
        console.warn('!~ invalid ' + dir + '-' + name + ':', v);
        show_range_err(dir, name, v, min, max);
        setTimeout(function() {
            console.debug('~ ' + dir + '-' + name + ' trigger blur:', value);
            $('#dial-' + dir + '-' + name).val(value);
            $('#dial-' + dir + '-' + name).trigger('blur')
        }, 1);
        return false;
    } else {
        return true;
    }
}

function show_range_err(dir, name, v, min, max) {
    var n = '';
    switch (name) {
    case 'sec':
        n = 'seconds';
        break;
    case 'size':
        n = 'packet size';
        break;
    case 'pkt':
        n = 'packets';
        break;
    case 'bw':
        n = 'bandwidth';
        break;
    }
    show_temp_err('Dial reset. It would cause the ' + n
            + ' dial to exceed its limit of ' + parseInt(max) + '.');
}

function show_temp_err(msg) {
    $('#error_text').removeClass('enable')
    $('#error_text').addClass('enable').text(msg);
    // remove animation once done
    $('#error_text').one('animationend', function(e) {
        $('#error_text').removeClass('enable').text('');
    });
}

function onchange_sec(dir, v, min, max, lock) {
    // changed sec, so update bw, else pkt
    if (src_in_range(dir, 'sec', v, min, max)) {
        var valid = true;
        switch (lock) {
        case 'size':
            valid = update_bw(dir);
            break;
        case 'pkt':
            valid = update_bw(dir);
            break;
        case 'bw':
            valid = update_pkt(dir);
            break;
        }
        if (!valid) {
            update_sec(dir);
        }
    }
    // special case: update continuous interval
    updateBwInterval();
}

function onchange_size(dir, v, min, max, lock) {
    // changed size, so update non-locked pkt/bw, feedback on max
    if (src_in_range(dir, 'size', v, min, max)) {
        var valid = true;
        switch (lock) {
        case 'size':
            valid = update_size(dir);
            break;
        case 'pkt':
            valid = update_bw(dir);
            break;
        case 'bw':
            valid = update_pkt(dir);
            break;
        }
        if (!valid) {
            update_size(dir);
        }
    }
}

function onchange_pkt(dir, v, min, max, lock) {
    // changed pkt, so update non-locked size/bw, feedback on max
    if (src_in_range(dir, 'pkt', v, min, max)) {
        var valid = true;
        switch (lock) {
        case 'size':
            valid = update_bw(dir);
            break;
        case 'pkt':
            valid = update_pkt(dir);
            break;
        case 'bw':
            valid = update_size(dir);
            break;
        }
        if (!valid) {
            update_pkt(dir);
        }
    }
}

function onchange_bw(dir, v, min, max, lock) {
    // changed bw, so update non-locked size/pkt, feedback on max
    if (src_in_range(dir, 'bw', v, min, max)) {
        var valid = true;
        switch (lock) {
        case 'size':
            valid = update_pkt(dir);
            break;
        case 'pkt':
            valid = update_size(dir);
            break;
        case 'bw':
            valid = update_bw(dir);
            break;
        }
        if (!valid) {
            update_bw(dir);
        }
    }
}

function update_sec(dir) {
    var val = parseInt(get_sec(dir) / 1000000);
    return update_dial(dir, 'sec', val, secMin, secMax);
}

function update_size(dir) {
    var val = parseInt(get_size(dir) * 1000000);
    return update_dial(dir, 'size', val, sizeMin, sizeMax);
}

function update_pkt(dir) {
    var val = parseInt(get_pkt(dir) * 1000000);
    return update_dial(dir, 'pkt', val, pktMin, pktMax);
}

function update_bw(dir) {
    var val = parseFloat(get_bw(dir) / 1000000);
    return update_dial(dir, 'bw', val, bwMin, bwMax);
}

function get_sec(dir) {
    return $('#dial-' + dir + '-pkt').val() * $('#dial-' + dir + '-size').val()
            * 8 / $('#dial-' + dir + '-bw').val();
}

function get_size(dir) {
    return $('#dial-' + dir + '-bw').val() / $('#dial-' + dir + '-pkt').val()
            * $('#dial-' + dir + '-sec').val() / 8;
}

function get_pkt(dir) {
    return $('#dial-' + dir + '-bw').val() / $('#dial-' + dir + '-size').val()
            * $('#dial-' + dir + '-sec').val() / 8;
}

function get_bw(dir) {
    return $('#dial-' + dir + '-pkt').val() * $('#dial-' + dir + '-size').val()
            / $('#dial-' + dir + '-sec').val() * 8;
}

function setDialLock(dir, value, readOnly) {
    var radioId = dir + '-radio-' + value;
    var dialId = 'dial-' + dir + '-' + value;
    $("#" + radioId).prop("checked", readOnly);
    $("#" + dialId).prop("readonly", readOnly);
    // $("#" + dialId).prop("disabled", readOnly);
    $("#" + dialId).trigger('configure', {
        "readOnly" : readOnly ? "true" : "false",
        "inputColor" : readOnly ? "#999" : "#000",
    });
    if (readOnly) {
        console.debug(dialId + ' locked');
    }
}
