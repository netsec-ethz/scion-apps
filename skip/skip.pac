function FindProxyForURL(url, host)
{
  let mungedScionAddr = /^\d+-[-_.\dA-Fa-f]+$/
  if (host.match(mungedScionAddr) != null || shExpMatch(host, "*.scion")) {
	  return "PROXY localhost:8888";
  }
  return "DIRECT";
}
