package term

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/howeyc/gopass"
)

func Read(prompt string) string {
	for {
		Infoln(prompt)

		var ans string
		fmt.Scanln(&ans)
		ans = strings.TrimSpace(ans)
		if ans != "" {
			return ans
		}
	}
}

func ReadMasked(prompt string) string {
	for {
		Infoln(prompt)
		if token, err := gopass.GetPasswdMasked(); err == nil {
			token = bytes.TrimSpace(token)
			if len(token) > 0 {
				return string(token)
			}
		}
	}
}

func Ask(prompt string, defaultYes bool) bool {
	for {
		if defaultYes {
			Infoln(prompt, "[Yes/no]")

			var ans string
			fmt.Scanln(&ans)
			ans = strings.TrimSpace(ans)
			if strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes") || ans == "" {
				return true
			}
			if strings.EqualFold(ans, "n") || strings.EqualFold(ans, "no") {
				return false
			}
		} else {
			Infoln(prompt, "[yes/No]")

			var ans string
			fmt.Scanln(&ans)
			ans = strings.TrimSpace(ans)
			if strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes") {
				return true
			}
			if strings.EqualFold(ans, "n") || strings.EqualFold(ans, "no") || ans == "" {
				return false
			}
		}
	}
}

func Confirm(prompt string) {
	Infoln(prompt, "[yes/No]")
	for {
		var ans string
		fmt.Scanln(&ans)
		ans = strings.TrimSpace(ans)
		if strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes") {
			return
		}
		if strings.EqualFold(ans, "n") || strings.EqualFold(ans, "no") || ans == "" {
			Infoln("Please complete the above step first. Are you done? [yes/No]")
		}
	}
}

func List(items []string) (int, string) {
	sort.Strings(items)
	width := len(strconv.Itoa(len(items)))
	for i, item := range items {
		fmt.Printf("[%"+strconv.Itoa(width)+"d] %v\n", i+1, item)
	}
	for {
		fmt.Print("Select option: ")
		var ans string
		fmt.Scanln(&ans)
		ans = strings.TrimSpace(ans)
		if i, err := strconv.Atoi(ans); err == nil {
			if i >= 1 && i <= len(items) {
				return i - 1, items[i-1]
			}
		}
	}
}
