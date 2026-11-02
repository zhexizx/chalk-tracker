package tui

import "charm.land/bubbles/v2/key"

type keyMap struct {
	Up, Down, Switch           key.Binding
	Add, Edit, Toggle, Delete  key.Binding
	Filter, Help, Quit, Cancel key.Binding
}

func newKeyMap() keyMap {
	binding := func(help, desc string, keys ...string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, desc))
	}
	return keyMap{
		Up:     binding("↑/k", "上移", "up", "k"),
		Down:   binding("↓/j", "下移", "down", "j"),
		Switch: binding("tab", "切换面板", "tab", "shift+tab", "left", "right", "h", "l"),
		Add:    binding("a", "添加", "a"),
		Edit:   binding("e", "编辑", "e"),
		Toggle: binding("space", "完成", "space", " "),
		Delete: binding("d", "删除", "d"),
		Filter: binding("/", "筛选", "/"),
		Help:   binding("?", "帮助", "?"),
		Quit:   binding("q", "退出", "q", "ctrl+c"),
		Cancel: binding("esc", "取消", "esc"),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Add, k.Toggle, k.Edit, k.Delete, k.Filter, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Switch},
		{k.Add, k.Edit, k.Toggle, k.Delete},
		{k.Filter, k.Cancel, k.Help, k.Quit},
	}
}
