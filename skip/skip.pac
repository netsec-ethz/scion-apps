const scionHosts = new Set([
{{range .SCIONHosts}}  "{{.|js}}",
{{end}}])

function FindProxyForURL(url, host)
{
  let mungedScionAddr = /^\d+-[-_.\dA-Fa-f]+$/
  if (host.match(mungedScionAddr) != null ||
      scionHosts.has(host)) {
	  return "PROXY {{.ProxyAddress|js}}";
  }
  return "DIRECT";
}
