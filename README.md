# `gfmxr`

A (<strong>G</strong>itHub-<strong>F</strong>lavored <strong>M</strong>arkdown
E<strong>x</strong>ample <strong>R</strong>unner).  Runs stuff inside code gates
(maybe!).  Potentially pronounceable as "Gif Mizzer", "Guh'fMaxxer" or "Guff
Mazer".

This project is not intended to fully replace the example code running capabilities
of any given language's tooling, but instead to fill a gap.  In the case of a Go
repository, for example, it can be handy to have some verifiable code examples
in the `README.md` *and* example functions in `*_test.go` files.

## Explicitly Supported Languages

- [bash](#bash)
- [go](#go)
- [java](#java)
- [javascript](#javascript)
- [json](#json)
- [python](#python)
- [ruby](#ruby)
- [shell](#shell)
- [sh](#sh)
- [zsh](#zsh)

### Bash

If a code example has a declared language of `bash`, then `gfxmr` will write
the source to a temporary file and run it via whatever executable is first in
line to respond to `bash`.

<!-- {
  "output": "Begot by all the supernova"
} -->
``` bash
for i in {97..99} ; do
  echo "$i problems" >&2
done

echo "Begot by all the supernova (${0})"
exit 0
```

### Go

If a code example has a declared language of `go` and the first line is `package
main`, then `gfmxr` will write the source to a temporary file, build it, and run
it.  It is worth noting that `go run` is *not* used, as this executes a child
process of its own, thus making process management and exit success detection
all the more complex.

<!-- {
  "output": "we could make.*sound"
} -->
``` go
package main

import (
  "fmt"
  "os"
)

func main() {
  fmt.Printf("---> %v\n", os.Args[0])
  fmt.Println("we could make an entire album out of this one sound")
}
```

### Java

If a code example has a declared language of `java` and a line matching `^public
class ([^ ]+)`, then `gfmxr` will write the source to a temporary file, build
it, and run the class by name.

<!-- {
  "output": "Awaken the hive"
} -->
``` java
public class GalacticPerimeter {
  public static void main(String[] args) {
    System.out.println(System.getenv("FILE"));
    System.out.println("Awaken the hive");
  }
}
```

### JavaScript (assumed node.js compatible)

If a code example has a declared language of `javascript`, then `gfmxr` will
write the source to a temporary file and run it via whatever executable is first
in line to respond to `node`.

<!-- {
  "output": "they won't stop at dancin"
} -->
``` javascript
var main = function() {
  console.log("they won't stop at dancin, no");
  console.log("they won't stop at dancin");
};

if (require.main == module) {
  main();
}
```

### JSON

If a code example has a declared language of `json`, then `gfmxr` will write the
source to a temporary file and "run" it via the `node` executable for
validation.

``` json
{
  "no": "output",
  "levels": [
    8000,
    9000,
    9001
  ]
}
```

### Python

If a code example has a declared language of `python`, then `gfxmr` will write
the source to a temporary file and run it via whatever executable is first in
line to respond to `python`.

<!-- {
  "output": "lipstick ringo dance all night \\['.*'\\]!"
} -->
``` python
from __future__ import print_function

import sys


def main():
    print('lipstick ringo dance all night {!r}!'.format(sys.argv))
    return 0


if __name__ == '__main__':
    sys.exit(main())
```

### Ruby

If a code example has a declared language of `ruby`, then `gfxmr` will write
the source to a temporary file and run it via whatever executable is first in
line to respond to `ruby`.

<!-- {
  "output": "get out of.*the king"
} -->
``` ruby

def main
  puts $0
  puts "get out of the path of the king"
  return 0
end

if $0 == __FILE__
  exit main
end
```

### Shell

If a code example has a declared language of `shell`, then `gfmxr` will write
the source to a temporary file and run it via whatever executable is first in
line to respond to `bash`.

<!-- {
  "output": "[jJ]ust the way you like it"
} -->
``` shell
if [ 0 -eq 0 ] ; then
  echo "Just the way you like it, yes"
  echo "just the way you like it. (${0})"
fi
exit 0
```

### Sh

If a code example has a declared language of `sh`, then `gfxmr` will write
the source to a temporary file and run it via whatever executable is first in
line to respond to `sh`.

<!-- {
  "output": "Saddle up preacher"
} -->
``` sh
if [ 5 -eq 3 ] ; then
  echo "YOU FOUND THE MARBLE IN THE OATMEAL"
else
  echo "Saddle up preacher, don't look back. (${0})"
fi
exit 0
```

### Zsh

If a code example has a declared language of `zsh`, then `gfxmr` will write
the source to a temporary file and run it via whatever executable is first in
line to respond to `zsh`.

<!-- {
  "output": "kiss me"
} -->
``` zsh
printf "Kiss me.\nJust kiss me.\n(${0})\n"

bye 0
```

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

``` node
var _ = 1 / 1;
```

Annotated with an `"output"` JSON tag that informs `gfxmr` to verify the example
program's output:

<!-- {
  "output": "Ohai from.*:wave:"
} -->

``` javascript
console.log("Ohai from the land of GitHub-Flavored Markdown :wave:");
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
