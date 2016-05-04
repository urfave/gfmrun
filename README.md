# `gfmxr`

A (<strong>G</strong>itHub-<strong>F</strong>lavored <strong>M</strong>arkdown
E<strong>x</strong>ample <strong>R</strong>unner).  Runs stuff inside code
gates (maybe!).  Potentially pronounceable as "Gif Mizzer", "Guh'fMaxxer" or
"Guff Mazer".

## Tag annotation comments

`gfmxr` supports the use of JSON tags embedded in comments preceding code
blocks, e.g. (just pretend `^` are backticks):

```
<!-- { "awesome": "maybe", "all on one line": "yep" } -->
^^^ go
package lolmain
// ... stuff
^^^
```

```
<!-- {
  "wow": "cool",
  "multiple": "lines"
} -->
^^^ go
package lolmain
// ... stuff
^^^
```

```
<!-- {
  "super": "advanced",
  "whitespace after the comment": "mindblown"
} -->


^^^ go
package lolmain
// ... stuff
^^^
```

### `"output"` tag

Given a regular expression string value, asserts that the program output
(stdout) matches.

### `"interrupt"` tag

Given either a truthy or duration string value, interrupts the program via
increasingly serious signals (`INT`, `HUP`, `TERM`, and finally `KILL`). If the
value is truthy, then the default duration is used (3s).  If the value is a
string duration, then the parsed duration is used.  This tag is intended for
use with long-lived example programs such as HTTP servers.

## Examples

No tag annotations, expected to be short-lived and exit successfully:

``` go
package main

func main() {
  _ = 1 / 1
}
```

Annotated with an `"output"` JSON tag that informs `gfxmr` to verify the example
program's output:

<!-- {
  "output": "Ohai from.*:wave:"
} -->

``` go
package main

import (
  "fmt"
)

func main() {
  fmt.Printf("Ohai from the land of GitHub-Flavored Markdown :wave:\n")
}
```

Annotated with an `"interrupt"` JSON tag that informs `gfmxr` to interrupt the
example program after a specified duration, which implies that the exit code is
ignored (not Windows-compatible):

<!-- { "interrupt": "2s" } -->
``` go
package main

import (
  "fmt"
  "net/http"
)

func main() {
  http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, "Why Hello From Your Friendly Server Example :bomb:\n")
  })
  http.ListenAndServe(":8990", nil)
}
```

Similar to the above example, but the `"interrupt"` tag is truthy rather than a
specific interval, which will result in the default interrupt duration being
used (3s).  It is also annotated with an `"output"` JSON tag that takes
precedence over the `"interrupt"` tag's behavior of ignoring the exit code:

<!-- {
  "interrupt": true,
  "output": ":bomb:"
} -->
``` go
package main

import (
  "fmt"
  "net/http"
)

func main() {
  http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, "Hello Again From Your Friendly Server Example :bomb:\n")
  })
  fmt.Println(":bomb:")
  http.ListenAndServe(":8989", nil)
}
```
