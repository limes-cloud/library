package get

import (
	"errors"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"reflect"
	"strings"
	"sync"
)

const (
	defaultDuplicatedKeyFormat = "{table}的{column}已存在:{value}"
	defaultAddForeignKeyFormat = "{table}的{column}不存在:{value}"
	defaultDelForeignKeyFormat = "{table}的{column}正在使用:{value}"
)

type GormErrorPlugin interface {
	Name() string
	Initialize(*gorm.DB) error
	Register(object TableComment)
	options() *options
}

type TableComment interface {
	Comment() string
}

type ErrorPlugin struct {
	opts   *options
	tables map[string]Info
	rw     sync.RWMutex
}

type Info struct {
	Name    string          `json:"name"`
	Comment string          `json:"comment"`
	Column  map[string]Info `json:"-" gorm:"-"`
}

func NewGormErrorPlugin(opts ...Option) GormErrorPlugin {
	o := &options{
		duplicatedKeyFormat: defaultDuplicatedKeyFormat,
		addForeignKeyFormat: defaultAddForeignKeyFormat,
		delForeignKeyFormat: defaultDelForeignKeyFormat,
	}
	for _, opt := range opts {
		opt(o)
	}
	return &ErrorPlugin{
		tables: map[string]Info{},
		rw:     sync.RWMutex{},
		opts:   o,
	}
}

func (ep *ErrorPlugin) Name() string {
	return "ErrorPlugin"
}

func (ep *ErrorPlugin) Initialize(db *gorm.DB) error {
	if ep.opts.enableLoad {
		ep.initLoad(db)
	}

	_ = db.Callback().Create().After("*").Register("transform:error", ep.Transform)
	_ = db.Callback().Query().After("*").Register("transform:error", ep.Transform)
	_ = db.Callback().Update().After("*").Register("transform:error", ep.Transform)
	_ = db.Callback().Delete().After("*").Register("transform:error", ep.Transform)
	_ = db.Callback().Row().After("*").Register("transform:error", ep.Transform)
	_ = db.Callback().Raw().After("*").Register("transform:error", ep.Transform)

	return nil
}

func (ep *ErrorPlugin) Transform(db *gorm.DB) {
	if db.Error == nil || db.Statement.Table == "" {
		return
	}

	// 使用预设的error
	if ep.opts.es[db.Error] != nil {
		db.Error = ep.opts.es[db.Error]
		return
	}

	// 不是mysql错误则不处理
	mErr := &mysql.MySQLError{}
	if !errors.As(db.Error, &mErr) {
		return
	}

	// 如果不是从数据库加载，则提前缓存一下
	//if !ep.opts.enableLoad {
	//	ep.addInfo(db)
	//}

	// 根据错误码处理错误
	switch mErr.Number {
	case 1062:
		db.Error = ep.DuplicatedKey(mErr)
	case 1452:
		db.Error = ep.ForeignKeyViolated(mErr)
	}
}

// Register 从外部注册
func (ep *ErrorPlugin) Register(object TableComment) {
	rv := reflect.ValueOf(object)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return
	}

	db := ep.opts.db
	table := db.NamingStrategy.TableName(rv.Type().Name())
	ep.tables[table] = Info{
		Column:  map[string]Info{},
		Name:    table,
		Comment: object.Comment(),
	}

	fieldNum := rv.NumField()
	for index := 0; index < fieldNum; index++ {
		ft := rv.Type().Field(index)

		comment := ""
		column := ""
		// 获取字段备注
		gormTag := ft.Tag.Get("gorm")
		//分割 `gorm` tag 以获取 comment 部分
		parts := strings.Split(gormTag, ";")
		for _, part := range parts {
			part = strings.Trim(part, " ")
			if strings.HasPrefix(part, "comment:") {
				comment = strings.Trim(strings.TrimPrefix(part, "comment:"), " ")
			}
			if strings.HasPrefix(part, "column:") {
				column = strings.Trim(strings.TrimPrefix(part, "column:"), " ")
			}
		}

		if ft.Tag.Get("comment") != "" {
			comment = ft.Tag.Get("comment")
		}

		if column == "" {
			column = db.NamingStrategy.ColumnName(table, ft.Name)
		}
		ep.tables[table].Column[column] = Info{
			Name:    column,
			Comment: comment,
		}
	}

}

func (ep *ErrorPlugin) options() *options {
	return ep.opts
}

// addInfo 重db中提取信息
func (ep *ErrorPlugin) addInfo(db *gorm.DB) {
	ep.rw.RLock()
	if _, is := ep.tables[db.Statement.Table]; is {
		ep.rw.RUnlock()
		return
	}
	ep.rw.RUnlock()

	if db.Statement.Schema == nil || len(db.Statement.Schema.Fields) == 0 {
		return
	}

	// 添加
	tableComment := ""
	if tc, ok := db.Statement.Model.(TableComment); ok {
		tableComment = tc.Comment()
	}

	ep.rw.Lock()
	defer ep.rw.Unlock()
	ep.tables[db.Statement.Table] = Info{Name: db.Statement.Table, Comment: tableComment, Column: map[string]Info{}}
	for _, field := range db.Statement.Schema.Fields {
		ep.tables[db.Statement.Table].Column[field.DBName] = Info{Name: field.DBName, Comment: field.Comment}
	}
}

// table 获取table comment
func (ep *ErrorPlugin) table(in string) string {
	ep.rw.RLock()
	defer ep.rw.RUnlock()

	cm := ep.tables[in].Comment
	if cm == "" {
		return ep.tables[in].Name
	}
	return cm
}

// column 获取column comment
func (ep *ErrorPlugin) column(tb, in string) string {
	ep.rw.RLock()
	defer ep.rw.RUnlock()

	cm := ep.tables[tb].Column[in].Comment
	if cm == "" {
		return ep.tables[tb].Column[in].Name
	}
	return cm
}

// DuplicatedKey 处理唯一索引错误
func (ep *ErrorPlugin) DuplicatedKey(err *mysql.MySQLError) error {
	parts := strings.Split(err.Message, "'")
	if len(parts) <= 3 {
		return err
	}

	fields := strings.Split(parts[3], ".")
	if len(fields) != 2 {
		return err
	}

	format := ep.opts.duplicatedKeyFormat
	// 替换table
	format = strings.Replace(format, "{table}", ep.table(fields[0]), 1)
	// 替换column
	format = strings.Replace(format, "{column}", ep.column(fields[0], fields[1]), 1)
	// 替换value
	format = strings.Replace(format, "{value}", parts[1], 1)

	return errors.New(format)
}

// ForeignKeyViolated 处理引用错误
func (ep *ErrorPlugin) ForeignKeyViolated(err *mysql.MySQLError) error {
	arr := strings.Split(err.Message, "REFERENCES")
	if len(arr) != 2 {
		return err
	}

	// 提取表和列
	parts := strings.Split(arr[1][1:], " (`")
	if len(parts) != 2 {
		return err
	}

	table := strings.Trim(parts[0], "`")
	column := strings.Trim(parts[1], "`)")

	format := ""
	if strings.Contains(err.Message, "Cannot add or update a child row") {
		format = ep.opts.addForeignKeyFormat
	}

	if strings.Contains(err.Message, "Cannot delete or update a parent row") {
		format = ep.opts.delForeignKeyFormat
	}
	// 替换table
	format = strings.Replace(format, "{table}", ep.table(table), 1)
	// 替换column
	format = strings.Replace(format, "{column}", ep.column(table, column), 1)
	// 替换value
	format = strings.Replace(format, "{value}", parts[1], 1)

	return errors.New(format)
}

// initLoad 从数据库获取comment
func (ep *ErrorPlugin) initLoad(db *gorm.DB) {
	// 获取全部表
	var tables []string
	db.Raw("show tables").Scan(&tables)
	database := db.Migrator().CurrentDatabase()
	for _, table := range tables {
		// 获取表comment
		tSql := "select table_name as name,table_comment as comment from information_schema.tables where table_schema = ? and table_name = ?"
		tableInfo := Info{}
		db.Raw(tSql, database, table).Scan(&tableInfo)

		cSql := "select column_name as name, column_comment as comment from information_schema.columns where table_schema = ? AND table_name = ?"
		columnInfos := make([]Info, 0)
		db.Raw(cSql, database, table).Scan(&columnInfos)

		// 关联表
		tableInfo.Column = map[string]Info{}
		for _, ci := range columnInfos {
			tableInfo.Column[ci.Name] = ci
		}
		ep.tables[table] = tableInfo
	}
}
