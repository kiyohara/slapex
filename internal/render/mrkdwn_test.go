package render

import "testing"

type fakeTextResolver struct {
	users map[string]string
	emoji map[string]string
}

func (r fakeTextResolver) UserName(id string) string {
	if name, ok := r.users[id]; ok {
		return name
	}
	return id
}

func (r fakeTextResolver) EmojiHTML(name string) string {
	if html, ok := r.emoji[name]; ok {
		return html
	}
	return ":" + name + ":"
}

func TestMrkdwn(t *testing.T) {
	resolver := fakeTextResolver{
		users: map[string]string{
			"U123": "alice",
			"W456": "bob",
		},
		emoji: map[string]string{
			"wave":   "👋",
			"custom": `<img class="emoji" src="assets/emoji/custom.png" alt=":custom:" title=":custom:">`,
		},
	}

	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "bold",
			text: "*bold*",
			want: "<strong>bold</strong>",
		},
		{
			name: "italic",
			text: "_italic_",
			want: "<em>italic</em>",
		},
		{
			name: "strike",
			text: "~strike~",
			want: "<s>strike</s>",
		},
		{
			name: "inline code",
			text: "`code`",
			want: "<code>code</code>",
		},
		{
			name: "code block",
			text: "```\nfirst line\nsecond line\n```",
			want: "<pre><code>first line\nsecond line</code></pre>",
		},
		{
			name: "quote block from slack escaped greater than",
			text: "&gt; quoted\nnormal",
			want: "<blockquote>quoted</blockquote><br>normal",
		},
		{
			name: "bare link",
			text: "<https://example.com/path?q=1>",
			want: `<a href="https://example.com/path?q=1">https://example.com/path?q=1</a>`,
		},
		{
			name: "labeled link",
			text: "<https://example.com|Example>",
			want: `<a href="https://example.com">Example</a>`,
		},
		{
			name: "user mention",
			text: "<@U123>",
			want: `<span class="mention">@alice</span>`,
		},
		{
			name: "user mention falls back to id",
			text: "<@U999>",
			want: `<span class="mention">@U999</span>`,
		},
		{
			name: "workspace user mention",
			text: "<@W456>",
			want: `<span class="mention">@bob</span>`,
		},
		{
			name: "channel mention",
			text: "<#C123|general>",
			want: `<span class="mention">#general</span>`,
		},
		{
			name: "special mentions",
			text: "<!here> <!channel> <!everyone>",
			want: `<span class="mention">@here</span> <span class="mention">@channel</span> <span class="mention">@everyone</span>`,
		},
		{
			name: "subteam mention",
			text: "<!subteam^S123|@eng>",
			want: `<span class="mention">@eng</span>`,
		},
		{
			name: "date token fallback",
			text: "<!date^1609459200^{date_short}|Jan 1, 2021>",
			want: "Jan 1, 2021",
		},
		{
			name: "unicode emoji",
			text: "hello :wave:",
			want: "hello 👋",
		},
		{
			name: "local image emoji",
			text: "ship it :custom:",
			want: `ship it <img class="emoji" src="assets/emoji/custom.png" alt=":custom:" title=":custom:">`,
		},
		{
			name: "line break",
			text: "one\ntwo",
			want: "one<br>two",
		},
		{
			name: "html fragment stays entity escaped",
			text: "&lt;script&gt;alert(1)&lt;/script&gt;",
			want: "&lt;script&gt;alert(1)&lt;/script&gt;",
		},
		{
			name: "non http scheme stays text",
			text: "<javascript:alert(1)>",
			want: "javascript:alert(1)",
		},
		{
			name: "href attribute quotes are escaped",
			text: `<https://example.com/?q="x'>`,
			want: `<a href="https://example.com/?q=&quot;x&#39;">https://example.com/?q="x'</a>`,
		},
		{
			name: "link label stays entity escaped",
			text: "<https://example.com|&lt;b&gt;label&lt;/b&gt;>",
			want: `<a href="https://example.com">&lt;b&gt;label&lt;/b&gt;</a>`,
		},
		{
			name: "inline code expands constructs as display text",
			text: "`<https://example.com|Example> <@U123> <#C123|general> <!here>`",
			want: "<code>Example @alice #general @here</code>",
		},
		{
			name: "code block expands link constructs as display text",
			text: "```\n<https://example.com>\n<https://example.com|Example>\n```",
			want: "<pre><code>https://example.com\nExample</code></pre>",
		},
		{
			name: "inline code keeps slack escaped entities without double escaping",
			text: "`&lt;literal&gt;`",
			want: "<code>&lt;literal&gt;</code>",
		},
		{
			name: "inline code escapes leftover raw less than",
			text: "`left < only`",
			want: "<code>left &lt; only</code>",
		},
		{
			name: "inline code escapes leftover raw greater than",
			text: "`right > only`",
			want: "<code>right &gt; only</code>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(Mrkdwn(tt.text, resolver)); got != tt.want {
				t.Fatalf("Mrkdwn(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
