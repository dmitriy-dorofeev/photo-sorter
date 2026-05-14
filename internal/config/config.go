// Package config содержит общие константы и значения по умолчанию,
// используемые в CLI и TUI для обеспечения согласованности.
package config

const (
	// DefaultTemplate — шаблон папок по умолчанию (Go time layout).
	DefaultTemplate = "2006-01-02"

	// DefaultFileNameTemplate — шаблон имён файлов по умолчанию (сохраняет оригинальное имя).
	DefaultFileNameTemplate = "{original}{ext}"

	// DefaultLivePhotos — группировать Live Photos по умолчанию.
	DefaultLivePhotos = true

	// DefaultIncludeVideo — обрабатывать видео по умолчанию.
	DefaultIncludeVideo = true

	// DefaultUseMTime — использовать дату изменения как fallback по умолчанию.
	DefaultUseMTime = true

	// DefaultDupStrategy — стратегия выбора оригинала из группы дубликатов.
	DefaultDupStrategy = "path"

	// DefaultCollisionStrategy — стратегия разрешения конфликтов имён.
	DefaultCollisionStrategy = "counter"

	// DefaultWriteExif — записывать определённую дату в EXIF по умолчанию.
	DefaultWriteExif = false

	// DefaultNotify — показывать системное уведомление по завершении.
	DefaultNotify = true
)
