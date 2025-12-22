package utils
import (
    "encoding/json"
    "strings"
)
func ExtractJSONFromContent(s string) string {
    t := strings.TrimSpace(s)
    if t == "" {
        return t
    }
    if json.Valid([]byte(t)) {
        return t
    }
    parts := strings.Split(t, "```")
    for _, p := range parts {
        c := strings.TrimSpace(p)
        lc := strings.ToLower(c)
        if strings.HasPrefix(lc, "json") {
            c = strings.TrimSpace(c[4:])
        }
        if c != "" && json.Valid([]byte(c)) {
            return c
        }
    }
    r := []rune(t)
    start := -1
    for i := 0; i < len(r); i++ {
        if r[i] == '{' || r[i] == '[' {
            start = i
            break
        }
    }
    if start >= 0 {
        var stack []rune
        inStr := false
        esc := false
        for j := start; j < len(r); j++ {
            ch := r[j]
            if inStr {
                if esc {
                    esc = false
                } else if ch == '\\' {
                    esc = true
                } else if ch == '"' {
                    inStr = false
                }
                continue
            }
            if ch == '"' {
                inStr = true
                continue
            }
            if ch == '{' || ch == '[' {
                stack = append(stack, ch)
            } else if ch == '}' || ch == ']' {
                if len(stack) == 0 {
                    continue
                }
                top := stack[len(stack)-1]
                if (top == '{' && ch == '}') || (top == '[' && ch == ']') {
                    stack = stack[:len(stack)-1]
                    if len(stack) == 0 {
                        cand := strings.TrimSpace(string(r[start : j+1]))
                        if json.Valid([]byte(cand)) {
                            return cand
                        }
                        break
                    }
                }
            }
        }
    }
    return t
}
