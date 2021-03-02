function FindProxyForURL(url, host)
{
  let mungedScionAddr = /^\d+-[-_.\dA-Fa-f]+$/
  if (host.match(mungedScionAddr) != null || shExpMatch(host, "*.scion")) {
	  return "PROXY {{.ProxyAddress}}";
  }
  return "DIRECT";
}
