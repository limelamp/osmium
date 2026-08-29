// Package assets holds the artwork the interface draws: the wordmark shown on
// the home page, in a few sizes.
package assets

import "strings"

// Logo1 is the wordmark in its widest form.
var Logo1 = ` ██████╗  ███████╗███╗   ███╗██╗██╗   ██╗███╗   ███╗
██╔═══██╗ ██╔════╝████╗ ████║██║██║   ██║████╗ ████║
██║   ██║ ███████╗██╔████╔██║██║██║   ██║██╔████╔██║
██║   ██║ ╚════██║██║╚██╔╝██║██║██║   ██║██║╚██╔╝██║
╚██████╔╝ ███████║██║ ╚═╝ ██║██║╚██████╔╝██║ ╚═╝ ██║
 ╚═════╝  ╚══════╝╚═╝     ╚═╝╚═╝ ╚═════╝ ╚═╝     ╚═╝`

// Logo2 is the wordmark the home page draws.
var Logo2 = ` ██████  ███████ ███    ███ ██ ██    ██ ███    ███
██    ██ ██      ████  ████ ██ ██    ██ ████  ████
██    ██ ███████ ██ ████ ██ ██ ██    ██ ██ ████ ██
██    ██      ██ ██  ██  ██ ██ ██    ██ ██  ██  ██
 ██████  ███████ ██      ██ ██  ██████  ██      ██`

// Logo3 is the compact wordmark, for narrow terminals.
var Logo3 = strings.ReplaceAll("  .g8\"\"8q.                               db                            \n.dP'    ~YM.                                                              \ndM'      ~MM ,pP\"Ybd ~7MMpMMMb.pMMMb.  ~7MM  ~7MMpMMMb.pMMMb.  \nMM        8I   ~\"   MM    MM.      ,MP ~YMMMa.   MM    ~Mb.    ,dP' L.   I8   MM    ~\"bmmd\"'   M9mmmP' .JMML  JMML  JMML..JMML. ~Mbod\"YML..JMML  JMML  JMML.", "~", "`")
