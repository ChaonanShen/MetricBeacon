package clocks

import "time"

type Clock interface{ Now() time.Time }
