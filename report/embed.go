package report

import _ "embed"

//go:embed assets/lightweight-charts.standalone.production.js
var lwcJS string

//go:embed templates/report.html.tmpl
var tmplText string

// LightweightChartsJS returns a copy of the embedded Lightweight Charts v5
// bundle shipped inside generated reports, so other frontends (e.g.
// cmd/webui) can serve the identical vendored asset without duplicating it.
func LightweightChartsJS() []byte { return []byte(lwcJS) }
