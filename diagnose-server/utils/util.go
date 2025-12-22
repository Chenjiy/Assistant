package utils

import (
    "encoding/json"
    "strings"
)

func ExtractJSONFromContent(s string) string {
    t := strings.TrimSpace(s)
    if json.Valid([]byte(t)) {
        return t
    }
    parts := strings.Split(t, "```")
    for _, p := range parts {
        c := strings.TrimSpace(p)
        if strings.HasPrefix(strings.ToLower(c), "json") {
            c = strings.TrimSpace(c[4:])
        }
        if c != "" && json.Valid([]byte(c)) {
            return c
        }
    }
    b := []rune(t)
    var i int
    for i = 0; i < len(b); i++ {
        if b[i] == '{' || b[i] == '[' {
            break
        }
    }
    if i < len(b) {
        var stack []rune
        inStr := false
        esc := false
        for j := i; j < len(b); j++ {
            r := b[j]
            if inStr {
                if !esc && r == '\\' {
                    esc = true
                } else {
                    if !esc && r == '"' {
                        inStr = false
                    }
                    esc = false
                }
            } else {
                if r == '"' {
                    inStr = true
                } else if r == '{' || r == '[' {
                    stack = append(stack, r)
                } else if r == '}' || r == ']' {
                    if len(stack) == 0 {
                        continue
                    }
                    top := stack[len(stack)-1]
                    if (top == '{' && r == '}') || (top == '[' && r == ']') {
                        stack = stack[:len(stack)-1]
                        if len(stack) == 0 {
                            cand := strings.TrimSpace(string(b[i : j+1]))
                            if json.Valid([]byte(cand)) {
                                return cand
                            }
                        }
                    }
                }
            }
        }
    }
    return t
}
