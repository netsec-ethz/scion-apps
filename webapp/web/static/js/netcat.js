var yourMsg, yourVideo, friendsVideo, logMsgs, socketT, socketV;

$(document).ready(function() {
    yourMsg = document.getElementById("yourMsg");
    yourVideo = document.getElementById("yourVideo");
    friendsVideo = document.getElementById("friendsVideo");
    logMsgs = document.getElementById("logMsgs");
});

function openNetcatChatText(localAddr, remoteAddr) {

    // TODO chrome throws an exception

    socketT = new WebSocket(encodeURI("ws://" + document.location.host
            + "/wschat?local=" + localAddr + "&remote=" + remoteAddr));

    socketT.onopen = function() {
        appendChatDisplay("Status: WS text opened\n");
    };

    socketT.onmessage = function(e) {
        appendChatDisplay("Friend: " + e.data + "\n");
    };
}

function openNetcatChatVideo(localAddr, remoteAddr) {
    socketV = new WebSocket(encodeURI("ws://" + document.location.host
            + "/wsvideo?local=" + localAddr + "&remote=" + remoteAddr));

    socketV.onopen = function() {
        debugLog("WS video opened\n");
    };

    socketV.onmessage = function(e) {
        debugLog("WS video incoming...\n");
        friendsVideo.srcObject = e.stream;
    };
}

function sendTextMsg() {
    if (socketT) {
        socketT.send(yourMsg.value);
        appendChatDisplay("You: " + yourMsg.value + "\n");
        yourMsg.value = "";
    }
}

function sendVideoStream(stream) {
    if (socketV) {
        socketV.send(stream);
    }
}

function appendChatDisplay(msg) {
    var doScroll = logMsgs.scrollTop > logMsgs.scrollHeight
            - logMsgs.clientHeight - 1;
    var item = document.createElement("div");
    item.innerHTML = msg;
    logMsgs.appendChild(item);
    if (doScroll) {
        logMsgs.scrollTop = logMsgs.scrollHeight - logMsgs.clientHeight;
    }
}
