package cmd

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			if c == '-' && n == 0 {
				continue
			}
			return 0, nil
		}
		n = n*10 + int(c-'0')
	}
	if len(s) > 0 && s[0] == '-' {
		n = -n
	}
	return n, nil
}
