package mutex

import "sync"

var Players = sync.RWMutex{}
var Viewers = sync.RWMutex{}
var Games = sync.RWMutex{}
