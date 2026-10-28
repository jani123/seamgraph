package ui

type splitRow struct {
	Before, After diffLine
	Header        string
}

func splitRows(view diffView) []splitRow {
	result := []splitRow{}
	for i := 0; i < len(view.Rows); {
		row := view.Rows[i]
		if row.Kind == "equal" {
			result = append(result, splitRow{Before: row, After: row})
			i++
			continue
		}
		if row.Kind != "removed" && row.Kind != "added" {
			result = append(result, splitRow{Header: row.Text})
			i++
			continue
		}
		left, right := []diffLine{}, []diffLine{}
		note := ""
		for i < len(view.Rows) && (view.Rows[i].Kind == "removed" || view.Rows[i].Kind == "added" || view.Rows[i].Kind == "meta") {
			line := view.Rows[i]
			if line.Kind == "meta" {
				note = line.Text
				i++
				continue
			}
			if line.Kind == "removed" {
				left = append(left, line)
			} else {
				right = append(right, line)
			}
			i++
		}
		for j := 0; j < max(len(left), len(right)); j++ {
			pair := splitRow{}
			if j < len(left) {
				pair.Before = left[j]
			}
			if j < len(right) {
				pair.After = right[j]
			}
			result = append(result, pair)
		}
		if note != "" {
			result = append(result, splitRow{Header: note})
		}
	}
	return result
}
