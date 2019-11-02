package mbpdecor

import (
	"sync"

	"github.com/vbauerster/mpb/v4/decor"
)

// Status returns updatable status decorator.
//
//  `status` status to display
//
//	`wcc`    optional WC config
func Status(status string, wcc ...decor.WC) *StatusDecorator {
	var wc decor.WC
	for _, widthConf := range wcc {
		wc = widthConf
	}
	wc.Init()
	d := &StatusDecorator{
		WC:     wc,
		status: &status,
	}
	return d
}

type StatusDecorator struct {
	decor.WC
	status     *string
	statusLock sync.Mutex
}

func (d *StatusDecorator) SetStatus(status string) {
	d.statusLock.Lock()
	defer d.statusLock.Unlock()
	d.status = &status
}

func (d *StatusDecorator) Decor(st *decor.Statistics) string {
	d.statusLock.Lock()
	defer d.statusLock.Unlock()
	return d.FormatMsg(*d.status)
}
