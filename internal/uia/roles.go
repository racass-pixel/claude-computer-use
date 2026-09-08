// Package uia implements platform.Accessibility with Windows UI Automation over raw COM (no cgo).
package uia

import "strings"

var roleIDs = map[string]int32{
	"button": 50000, "calendar": 50001, "checkbox": 50002, "combobox": 50003, "edit": 50004,
	"hyperlink": 50005, "image": 50006, "listitem": 50007, "list": 50008, "menu": 50009,
	"menubar": 50010, "menuitem": 50011, "progressbar": 50012, "radiobutton": 50013,
	"scrollbar": 50014, "slider": 50015, "spinner": 50016, "statusbar": 50017, "tab": 50018,
	"tabitem": 50019, "text": 50020, "toolbar": 50021, "tooltip": 50022, "tree": 50023,
	"treeitem": 50024, "custom": 50025, "group": 50026, "thumb": 50027, "datagrid": 50028,
	"dataitem": 50029, "document": 50030, "splitbutton": 50031, "window": 50032, "pane": 50033,
	"header": 50034, "headeritem": 50035, "table": 50036, "titlebar": 50037, "separator": 50038,
}

var roleNames = map[int32]string{}

func init() {
	pretty := map[string]string{
		"checkbox": "CheckBox", "combobox": "ComboBox", "listitem": "ListItem", "menubar": "MenuBar",
		"menuitem": "MenuItem", "progressbar": "ProgressBar", "radiobutton": "RadioButton",
		"scrollbar": "ScrollBar", "statusbar": "StatusBar", "tabitem": "TabItem", "toolbar": "ToolBar",
		"tooltip": "ToolTip", "treeitem": "TreeItem", "datagrid": "DataGrid", "dataitem": "DataItem",
		"splitbutton": "SplitButton", "headeritem": "HeaderItem", "titlebar": "TitleBar",
	}
	for k, v := range roleIDs {
		name, ok := pretty[k]
		if !ok {
			name = strings.ToUpper(k[:1]) + k[1:]
		}
		roleNames[v] = name
	}
}

func RoleID(name string) (int32, bool) {
	id, ok := roleIDs[strings.ToLower(strings.ReplaceAll(name, " ", ""))]
	return id, ok
}

func RoleName(id int32) string {
	if n, ok := roleNames[id]; ok {
		return n
	}
	return "Custom"
}
