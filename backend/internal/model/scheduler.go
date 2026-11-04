package model

import "time"

// SchedulerJob is the shared shape of parity, mover and TRIM schedules.
type SchedulerJob struct {
	Mode          string `json:"mode"`
	Hour          int    `json:"hour"`
	Minute        int    `json:"minute"`
	Day           int    `json:"day"`
	DayOfMonth    int    `json:"dayOfMonth"`
	Month         int    `json:"month"`
	IntervalHours int    `json:"intervalHours"`
}

type ParitySchedule struct {
	SchedulerJob
	WriteCorrections bool `json:"writeCorrections"`
}

type MoverSchedule struct {
	SchedulerJob
	Logging  bool `json:"logging"`
	Progress bool `json:"progress"`
}

type TrimSchedule struct {
	SchedulerJob
}

type SchedulerSettings struct {
	Parity ParitySchedule `json:"parity"`
	Mover  MoverSchedule  `json:"mover"`
	Trim   TrimSchedule   `json:"trim"`
}

// SystemDisplaySettings are the two official display fields Aurora writes.
type SystemDisplaySettings struct {
	DateFormat string `json:"dateFormat"`
	TimeFormat string `json:"timeFormat"`
}

type SystemSettings struct {
	TimeZone    string                `json:"timeZone"`
	NTPEnabled  bool                  `json:"ntpEnabled"`
	NTPServers  []string              `json:"ntpServers"`
	CurrentTime time.Time             `json:"currentTime"`
	Display     SystemDisplaySettings `json:"display"`
}

type SchedulerState struct {
	Settings SchedulerSettings `json:"settings"`
	System   SystemSettings    `json:"system"`
}

type SchedulerPreviewFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Diff    string `json:"diff"`
	Existed bool   `json:"existed"`
	Delete  bool   `json:"delete"`
}

type SchedulerPreview struct {
	Files    []SchedulerPreviewFile `json:"files"`
	Warnings []string               `json:"warnings"`
}

type SchedulerApply struct {
	SchedulerSettings
	Acknowledge bool `json:"acknowledge"`
}

type DisplayApply struct {
	SystemDisplaySettings
	Acknowledge bool `json:"acknowledge"`
}
