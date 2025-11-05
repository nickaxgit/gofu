package mutex

import "sync"

var Players = sync.RWMutex{}
var Devices = sync.RWMutex{}
var Games = sync.RWMutex{}
