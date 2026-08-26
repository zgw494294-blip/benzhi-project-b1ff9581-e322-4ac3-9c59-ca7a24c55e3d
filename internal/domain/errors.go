package domain

import "errors"

var ErrNotFound = errors.New("未找到记录")
var ErrIdempotent = errors.New("幂等命令已处理")
