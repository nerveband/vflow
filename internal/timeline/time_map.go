package timeline

import "sort"

type DeleteRange struct {
	StartFrame int `json:"start_frame"`
	EndFrame   int `json:"end_frame"`
}

type Map struct {
	DurationFrames int           `json:"duration_frames"`
	Deletes        []DeleteRange `json:"deletes"`
}

func BuildTimeMap(deletes []DeleteRange, durationFrames int) Map {
	sort.Slice(deletes, func(i, j int) bool {
		return deletes[i].StartFrame < deletes[j].StartFrame
	})
	return Map{DurationFrames: durationFrames, Deletes: deletes}
}

func (m Map) SourceToTimeline(sourceFrame int) (int, bool) {
	removed := 0
	for _, del := range m.Deletes {
		if sourceFrame >= del.StartFrame && sourceFrame < del.EndFrame {
			return 0, false
		}
		if sourceFrame >= del.EndFrame {
			removed += del.EndFrame - del.StartFrame
		}
	}
	return sourceFrame - removed, true
}

func (m Map) SourceBoundaryToTimeline(sourceFrame int) int {
	removed := 0
	for _, del := range m.Deletes {
		if sourceFrame <= del.StartFrame {
			break
		}
		if sourceFrame <= del.EndFrame {
			return del.StartFrame - removed
		}
		removed += del.EndFrame - del.StartFrame
	}
	return sourceFrame - removed
}
