Ultimately this code will 'SFU' video (and audio) between thousands of connected parties.
At some point it will need to scale out horizontally - but that is relatively straighforward, and we are *far* from needing it yet
Fairly sure we could host an event for ~1000 users on a single machine without doing anything too exotic


Code is based on the article:-

https://www.piesocket.com/blog/golang-websocket

I'm sure it will need to morph a lot over time - but it is possible to hande 100's of 1000's of sockets on a single golang server (a million is claimed possible - but I think that's just silly)

Concurrency (multithreading) is/was the biggest issue - this article was a big help - mutexes are the 'answer'
https://medium.com/swlh/handle-concurrency-in-gorilla-web-sockets-ade4d06acd9c


Go is a beatiful, simple language - which compiles to a single native executable for every major OS
It has packages - but not dependencies (in the sense that you don't have to bundle or deploy the packages, they are compiled in)

It is fast, stable and secure - developed by some of the smartest people on the planet (including the inventor of C)

It's not *easy* to write .. but it is small enough that in a couple of weeks you can grasp all the fundamentals
Two good videos by Jake Wright on YouTube got me started :-

Learn Go in 12 minutes:-
https://www.youtube.com/watch?v=C8LgvuEBraI&t=713s

and (less important early on)
Concurrency in Go
https://www.youtube.com/watch?v=LvgVSSpwND8&t=2s

It cherry-picks the best of JavaScript and and C - It's not really Object Oriented - but there are elegant ways to acheive the same sort of structure

The documentation is generally excellent - https://go.dev/ 

It produces, tiny, bulletproof .EXEs that just run for months on end

Don't get me wrong - Rust looks very cool too.. but I would say it has a steeper learning curve.. especially for juniors, it's a 'bigger' language


to build the project

go build gofu.go

(the extension is important!)


