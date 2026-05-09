package accounts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/config"
)

func (h *Handler) listAccounts(w http.ResponseWriter, r *http.Request) {
	page := intFromQuery(r, "page", 1)
	pageSize := intFromQuery(r, "page_size", 10)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 1
	}
	if pageSize > 5000 {
		pageSize = 5000
	}
	accounts := h.Store.Snapshot().Accounts
	reverseAccounts(accounts)
	q := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("q")))
	if q != "" {
		filtered := make([]config.Account, 0, len(accounts))
		for _, acc := range accounts {
			id := strings.ToLower(acc.Identifier())
			if strings.Contains(id, q) ||
				strings.Contains(strings.ToLower(acc.Name), q) ||
				strings.Contains(strings.ToLower(acc.Remark), q) ||
				strings.Contains(strings.ToLower(acc.Email), q) ||
				strings.Contains(strings.ToLower(acc.Mobile), q) {
				filtered = append(filtered, acc)
			}
		}
		accounts = filtered
	}
	total := len(accounts)
	totalPages := 1
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := make([]map[string]any, 0, end-start)
	for _, acc := range accounts[start:end] {
		testStatus, _ := h.Store.AccountTestStatus(acc.Identifier())
		token := strings.TrimSpace(acc.Token)
		items = append(items, map[string]any{
			"identifier":    acc.Identifier(),
			"name":          acc.Name,
			"remark":        acc.Remark,
			"email":         acc.Email,
			"mobile":        acc.Mobile,
			"proxy_id":      acc.ProxyID,
			"has_password":  acc.Password != "",
			"has_token":     token != "",
			"token_preview": maskSecretPreview(token),
			"test_status":   testStatus,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize, "total_pages": totalPages})
}

func (h *Handler) addAccount(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	_ = json.NewDecoder(r.Body).Decode(&req)
	acc := toAccount(req)
	if acc.Identifier() == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "需要 email 或 mobile"})
		return
	}
	err := h.Store.Update(func(c *config.Config) error {
		if acc.ProxyID != "" {
			if _, ok := findProxyByID(*c, acc.ProxyID); !ok {
				return fmt.Errorf("代理不存在")
			}
		}
		mobileKey := config.CanonicalMobileKey(acc.Mobile)
		for _, a := range c.Accounts {
			if acc.Email != "" && a.Email == acc.Email {
				return fmt.Errorf("邮箱已存在")
			}
			if mobileKey != "" && config.CanonicalMobileKey(a.Mobile) == mobileKey {
				return fmt.Errorf("手机号已存在")
			}
		}
		c.Accounts = append(c.Accounts, acc)
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
		return
	}
	h.Pool.Reset()
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "total_accounts": len(h.Store.Snapshot().Accounts)})
}

func (h *Handler) updateAccount(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if decoded, err := url.PathUnescape(identifier); err == nil {
		identifier = decoded
	}

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}
	name, nameOK := fieldStringOptional(req, "name")
	remark, remarkOK := fieldStringOptional(req, "remark")

	err := h.Store.Update(func(c *config.Config) error {
		for i, acc := range c.Accounts {
			if !accountMatchesIdentifier(acc, identifier) {
				continue
			}
			if nameOK {
				c.Accounts[i].Name = name
			}
			if remarkOK {
				c.Accounts[i].Remark = remark
			}
			return nil
		}
		return newRequestError("账号不存在")
	})
	if err != nil {
		if detail, ok := requestErrorDetail(err); ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"detail": detail})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "total_accounts": len(h.Store.Snapshot().Accounts)})
}

func (h *Handler) batchImportAccounts(w http.ResponseWriter, r *http.Request) {
	type importResult struct {
		Line    int    `json:"line"`
		Email   string `json:"email"`
		Success bool   `json:"success"`
		Detail  string `json:"detail,omitempty"`
	}

	var req map[string]any
	_ = json.NewDecoder(r.Body).Decode(&req)
	text := strings.TrimSpace(fieldString(req, "text"))
	if text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "文本内容不能为空"})
		return
	}

	lines := strings.Split(text, "\n")
	var results []importResult
	addedCount := 0
	skippedCount := 0
	errorCount := 0

	processLine := func(lineNum int, line string) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			return
		}

		parts := strings.SplitN(line, "----", 2)
		if len(parts) != 2 {
			results = append(results, importResult{
				Line:    lineNum,
				Email:   line,
				Success: false,
				Detail:  "格式错误，需要 email----password 格式",
			})
			errorCount++
			return
		}

		email := strings.TrimSpace(parts[0])
		password := strings.TrimSpace(parts[1])
		if email == "" || password == "" {
			results = append(results, importResult{
				Line:    lineNum,
				Email:   email,
				Success: false,
				Detail:  "邮箱或密码为空",
			})
			errorCount++
			return
		}

		acc := config.Account{
			Name:     email,
			Email:    email,
			Password: password,
		}

		err := h.Store.Update(func(c *config.Config) error {
			for _, a := range c.Accounts {
				if a.Email == email {
					return fmt.Errorf("邮箱已存在")
				}
			}
			c.Accounts = append(c.Accounts, acc)
			return nil
		})

		if err != nil {
			errMsg := err.Error()
			if errMsg == "邮箱已存在" {
				results = append(results, importResult{
					Line:    lineNum,
					Email:   email,
					Success: false,
					Detail:  "重复，已跳过",
				})
				skippedCount++
			} else {
				results = append(results, importResult{
					Line:    lineNum,
					Email:   email,
					Success: false,
					Detail:  errMsg,
				})
				errorCount++
			}
			return
		}

		results = append(results, importResult{
			Line:    lineNum,
			Email:   email,
			Success: true,
		})
		addedCount++
	}

	for i, line := range lines {
		processLine(i+1, line)
	}

	h.Pool.Reset()
	writeJSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"total_lines":   len(lines),
		"added":         addedCount,
		"skipped":       skippedCount,
		"errors":        errorCount,
		"total_accounts": len(h.Store.Snapshot().Accounts),
		"results":       results,
	})
}

func (h *Handler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	identifier := chi.URLParam(r, "identifier")
	if decoded, err := url.PathUnescape(identifier); err == nil {
		identifier = decoded
	}
	err := h.Store.Update(func(c *config.Config) error {
		idx := -1
		for i, a := range c.Accounts {
			if accountMatchesIdentifier(a, identifier) {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("账号不存在")
		}
		c.Accounts = append(c.Accounts[:idx], c.Accounts[idx+1:]...)
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": err.Error()})
		return
	}
	h.Pool.Reset()
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "total_accounts": len(h.Store.Snapshot().Accounts)})
}
