package ui

import (
    "io/fs"
    ocdgui "app"
)

// GetWebFS returns the embedded web filesystem from the root embed
func GetWebFS() fs.FS { return ocdgui.WebFS() }


