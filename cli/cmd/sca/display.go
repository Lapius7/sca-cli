package main

import "fmt"

// printWhoamiFull は`sca whoami`のメイン出力。Lapount管理画面(/admin/users/:id)が
// オーナー向けに見せているのと同水準の情報(基本情報・MFA状態・連携プロバイダ全件)を
// ローカルCLI向けに再現する。
func printWhoamiFull(d *UserDetail, p Profile) {
	sectionTitle("👤", "ログイン中のユーザー")
	fmt.Println()

	name := p.DisplayName
	if name == "" {
		name = d.UserMetadata.FullName
	}
	if name == "" {
		name = d.UserMetadata.Name
	}
	if name != "" {
		printField("表示名", name)
	} else {
		printField("表示名", dim("(未設定)"))
	}

	if p.Handle != "" {
		printField("ハンドル", "@"+p.Handle)
	} else {
		printField("ハンドル", dim("(未設定)"))
	}

	emailLine := d.Email
	if d.EmailConfirmedAt != "" {
		emailLine += "  " + green("✓ 確認済み")
	}
	printField("メール", emailLine)
	printField("ユーザーID", dim(d.ID))
	if p.Locale != "" || p.Timezone != "" {
		printField("ロケール", dim(p.Locale+" / "+p.Timezone))
	}
	fmt.Println()

	printField("登録日時", formatDate(d.CreatedAt))
	printField("最終ログイン", formatDate(d.LastSignInAt))
	fmt.Println()

	printMFASection(d.Factors)
	fmt.Println()

	printProvidersSection(d.Identities)
}

func printMFASection(factors []MFAFactor) {
	if len(factors) == 0 {
		printField("2段階認証(MFA)", dim("未設定"))
		return
	}
	for i, f := range factors {
		label := "2段階認証(MFA)"
		if i > 0 {
			label = ""
		}
		icon, statusText := yellow("○"), f.Status
		if f.Status == "verified" {
			icon, statusText = green("✓"), "有効"
		}
		name := f.FriendlyName
		if name == "" {
			name = f.FactorType
		}
		extra := fmt.Sprintf("(%s、最終検証: %s)", name, formatDate(f.LastChallengedAt))
		printField(label, fmt.Sprintf("%s %s %s", icon, statusText, dim(extra)))
	}
}

func printProvidersSection(identities []Identity) {
	fmt.Println(bold(fmt.Sprintf("連携プロバイダ (%d)", len(identities))))
	if len(identities) == 0 {
		fmt.Println(dim("  連携なし"))
		return
	}
	for _, ident := range identities {
		fmt.Printf("  %s  %s  %s\n",
			padVisible(providerBadge(ident.Provider), providerBadgeWidth),
			ident.Email,
			dim("連携: "+formatDate(ident.CreatedAt)))
	}
}

// providerBadgeWidth はバッジ("● Google"等)の桁揃え基準幅。最長ラベル"X (Twitter)"を基準にする。
const providerBadgeWidth = 14

// padVisible は色コードを含む文字列を、色コードを除いた表示幅基準でpadする。
func padVisible(s string, target int) string {
	visible := stripANSI(s)
	w := displayWidth(visible)
	if w >= target {
		return s
	}
	return s + spaces(target-w)
}

func stripANSI(s string) string {
	out := make([]rune, 0, len(s))
	inEscape := false
	for _, r := range s {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

// printLoginSummary は`sca login`成功直後の短いサマリー(whoamiのフル版より簡潔に)。
func printLoginSummary(d *UserDetail, p Profile) {
	fmt.Println()
	name := p.DisplayName
	if name == "" {
		name = d.UserMetadata.FullName
	}
	if p.Handle != "" {
		if name != "" {
			fmt.Printf("  %s %s\n", bold(name), dim("@"+p.Handle))
		} else {
			fmt.Printf("  %s\n", bold("@"+p.Handle))
		}
	} else if name != "" {
		fmt.Printf("  %s\n", bold(name))
	}
	if d.Email != "" {
		fmt.Printf("  %s\n", dim(d.Email))
	}
	if len(d.Identities) > 0 {
		badges := make([]string, 0, len(d.Identities))
		for _, ident := range d.Identities {
			badges = append(badges, providerBadge(ident.Provider))
		}
		fmt.Printf("  %s ", dim("連携:"))
		for i, b := range badges {
			if i > 0 {
				fmt.Print("  ")
			}
			fmt.Print(b)
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Println(dim("  詳細は `sca whoami` で確認できます。"))
}
