const scionHosts = new Set([
{{range .SCIONHosts}}  "{{.}}",
{{end}}])

function FindProxyForURL(url, host)
{
  let mungedScionAddr = /^\d+-[-_.\dA-Fa-f]+$/
  if (host.match(mungedScionAddr) != null ||
      scionHosts.has(host)) {
	  return "PROXY {{.ProxyAddress}}";
  }
  return "DIRECT";
}
