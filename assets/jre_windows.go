//go:build windows

package assets

import _ "embed"

//go:embed jre-windows-x64.zip
var jreZip []byte

func init() { JRE = jreZip }
