//go:build darwin

package notify

import _ "embed"

//go:embed terminal-notifier.zip
var terminalNotifierZip []byte
