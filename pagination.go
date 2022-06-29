package dorm

import (
	"github.com/Masterminds/squirrel"
)

var defaultPageSize = uint64(20)
var defaultCurrent = uint64(1)

type Pagination struct {
	Current  *uint64 `json:"current"  desc:"当前页"`
	PageSize *uint64 `json:"pageSize" desc:"每页数量"`
	Total    *int64  `json:"total"    desc:"总数"`
}

func (p *Pagination) current() uint64 {
	if p.PageSize == nil || p.Current == nil {
		return defaultCurrent
	}
	var cur = &defaultCurrent
	if p.Current != nil {
		cur = p.Current
	}
	return *cur
}

//每页显示的数量
func (p *Pagination) limit() uint64 {
	if p.PageSize == nil || p.Current == nil {
		return defaultPageSize
	}
	var ps = &defaultPageSize
	if p.PageSize != nil {
		ps = p.PageSize
	}
	return *ps
}

//跳过的数量
func (p *Pagination) offset() uint64 {
	if p.PageSize == nil || p.Current == nil {
		return 0
	}
	return (p.current() - 1) * p.limit()
}

func (p *Pagination) Limit(q squirrel.SelectBuilder) squirrel.SelectBuilder {
	if p.PageSize == nil || p.Current == nil {
		return q
	}
	if p.required() {
		return q.Offset(p.offset()).Limit(p.limit())
	}
	return q
}

func (p *Pagination) GetDefault(total int64) *Pagination {
	if p.PageSize == nil || p.Current == nil {
		return &Pagination{Current: &defaultCurrent, PageSize: &defaultPageSize, Total: &total}
	}

	var c = p.current()
	var l = p.limit()
	return &Pagination{Current: &c, PageSize: &l, Total: &total}
}

func (p *Pagination) required() bool {
	if p.PageSize == nil || p.Current == nil {
		return false
	}
	return *p.Current > 0 && *p.PageSize > 0
}

func (p *Pagination) OrDefault() *Pagination {
	if p.PageSize == nil || p.Current == nil {
		return &Pagination{Current: &defaultCurrent, PageSize: &defaultPageSize}
	}

	return &Pagination{Current: p.Current, PageSize: p.PageSize}
}