package main

import (
	"context"
	"fmt"
	"time"

	"dnsway-pc/internal/engine"
	"dnsway-pc/internal/models"
)

func main() {
	// 1. 初始化引擎
	pcEngine := engine.NewEngine()
	ctx := context.Background()
	profileID := "user_child_001"

	// 2. 模拟一组 DNS 查询请求
	testDomains := []string{
		"www.google.com",     // 预期：REDIRECT (SafeSearch)
		"pornhub.com",        // 预期：BLOCK (Category: CAT_001)
		"roblox.com",         // 预期：BLOCK (Outside Time Window)
		"www.apple.com",      // 预期：ALLOW (Default)
		"doubleclick.net",    // 预期：BLOCK (Category: CAT_008 - Malware)
	}

	fmt.Println("=== DNS 家长控制系统 模拟运行 ===")
	fmt.Printf("当前时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("用户ID: %s\n\n", profileID)

	for _, domain := range testDomains {
		decision := pcEngine.Decide(ctx, profileID, domain, "")
		printDecision(decision)
	}
}

func printDecision(d *models.FilterDecision) {
	status := ""
	switch d.Decision {
	case models.Allow:
		status = "✅ [ALLOW]"
	case models.Block:
		status = "❌ [BLOCK]"
	case models.Redirect:
		status = "🔄 [REDIRECT]"
	}

	fmt.Printf("域名: %-20s | 结果: %-12s | 原因: %-30s | 耗时: %v\n",
		d.Domain,
		status,
		d.Reason,
		d.Latency,
	)
}