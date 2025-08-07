package ui

import (
    "io/fs"
    ocdgui "ocd-gui"
)

// GetWebFS returns the embedded web filesystem from the root embed
func GetWebFS() fs.FS { return ocdgui.WebFS() }


