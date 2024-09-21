package queue

import "time"

type delayItem struct {
	data interface{}
	time time.Time
}

func NewDelayItem(data interface{}, time time.Time) delayItem {
	return delayItem{data: data, time: time}
}

func (d *delayItem) GetData() interface{} {
	return d.data
}
