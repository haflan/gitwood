# gitwood
`gitwood` implements a limited set of read-only functionality from `git`.
It aims to be a dependency-free alternative to [go-git](https://github.com/go-git/go-git) for simple use cases.
Its design prioritizes thread-safety over performance, and leaves caching to the
user.
