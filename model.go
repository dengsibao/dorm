package dorm

import (
	"database/sql"
	"time"
)

type Model struct {
	Id        int64        `orm:"id,PRIMARY_KEY,AUTO_INCREMENT" json:"id"`
	CreatedAt time.Time    `orm:"created_at" json:"createdAt" desc:"创建时间"`
	UpdatedAt time.Time    `orm:"updated_at,NULL" json:"updatedAt" desc:"更新时间"`
	Deleted   bool         `orm:"deleted"    json:"-" desc:"是否删除(软删除)"`
	DeletedAt sql.NullTime `orm:"deleted_at,NULL" json:"-" desc:"删除时间"`
}

