const { app, BrowserWindow, dialog, Menu } = require('electron');
const path = require('path');
const fs = require('fs');
const os = require('os');
const { spawn, exec } = require('child_process');
const net = require('net');
const waitOn = require('wait-on');
const sudo = require('sudo-prompt');

let backendProcess = null;

const isDev = !app.isPackaged || process.env.NODE_ENV === 'development';

// 获取用户数据目录：~/.0e7/
function getUserDataDir() {
  const homeDir = os.homedir();
  const dataDir = path.join(homeDir, '.0e7');
  
  // 确保目录存在
  if (!fs.existsSync(dataDir)) {
    fs.mkdirSync(dataDir, { recursive: true });
  }
  
  return dataDir;
}

const PLATFORM_MAP = {
  darwin: {
    arm64: '0e7_darwin_arm64',
    x64: '0e7_darwin_amd64'
  },
  linux: {
    x64: '0e7_linux_amd64'
  },
  win32: {
    x64: '0e7_windows_amd64.exe'
  }
};

function resolveBinaryName() {
  const arch = process.arch === 'arm64' ? 'arm64' : 'x64';
  const platformBinaries = PLATFORM_MAP[process.platform];
  if (!platformBinaries) {
    throw new Error(`当前平台 ${process.platform} 暂不支持 Electron 打包`);
  }
  const binary = platformBinaries[arch] || platformBinaries.x64;
  if (!binary) {
    throw new Error(`未找到平台 ${process.platform}/${arch} 的二进制名称映射`);
  }
  return binary;
}

async function findFreePort(min = 45000, max = 55000, retries = 50) {
  const tryPort = () =>
    Math.floor(Math.random() * (max - min + 1)) + min;

  return new Promise((resolve, reject) => {
    const attempt = (remaining) => {
      if (remaining <= 0) {
        reject(new Error('无法找到可用端口'));
        return;
      }
      const port = tryPort();
      const server = net.createServer();
      server.unref();
      server.on('error', () => {
        server.close();
        attempt(remaining - 1);
      });
      server.listen(port, () => {
        server.close(() => resolve(port));
      });
    };
    attempt(retries);
  });
}

function resolveBinaryPath() {
  const binaryName = resolveBinaryName();
  if (isDev) {
    return path.resolve(__dirname, '..', '..', binaryName);
  }
  return path.join(process.resourcesPath, 'bin', binaryName);
}

function resolveConfigPath() {
  // 配置文件放在用户目录的 .0e7/ 目录中
  return path.join(getUserDataDir(), 'config.ini');
}

function attachLogging(child, label) {
  if (!child) {
    return;
  }
  child.stdout?.on('data', (data) => {
    console.log(`[${label}] ${data}`.trim());
  });
  child.stderr?.on('data', (data) => {
    console.error(`[${label}][err] ${data}`.trim());
  });
}

// 检测当前是否有管理员权限
async function checkAdminPrivileges() {
  return new Promise((resolve) => {
    const platform = process.platform;
    
    if (platform === 'win32') {
      // Windows: 使用 net session 命令检测
      exec('net session', (error) => {
        resolve(error === null);
      });
    } else if (platform === 'darwin' || platform === 'linux') {
      // macOS/Linux: 使用 id -u 检测是否为 root (uid 0)
      exec('id -u', (error, stdout) => {
        if (error) {
          resolve(false);
        } else {
          resolve(stdout.trim() === '0');
        }
      });
    } else {
      resolve(false);
    }
  });
}

// 尝试以管理员权限启动后端
async function launchBackendWithElevation(port) {
  const binaryPath = resolveBinaryPath();
  if (!fs.existsSync(binaryPath)) {
    throw new Error(`未找到 0E7 二进制文件: ${binaryPath}`);
  }
  
  const userDataDir = getUserDataDir();
  const configPath = resolveConfigPath();
  const args = ['--server', '--config', configPath, '--server-port', port.toString()];
  
  const platform = process.platform;
  let command;
  
  if (platform === 'win32') {
    // Windows: 使用 PowerShell 的 Start-Process 以管理员权限运行
    // 注意：需要转义路径和参数
    const escapedBinaryPath = binaryPath.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
    const escapedArgs = args.map(arg => `"${arg.replace(/"/g, '\\"')}"`).join(' ');
    const escapedCwd = userDataDir.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
    command = `powershell -Command "Start-Process -FilePath '${escapedBinaryPath}' -ArgumentList '${escapedArgs}' -WorkingDirectory '${escapedCwd}' -Verb RunAs -WindowStyle Hidden"`;
  } else {
    // macOS/Linux: 使用 sudo，设置工作目录和环境变量
    const envVars = `OE7_SERVER_PORT=${port.toString()}`;
    command = `cd "${userDataDir}" && ${envVars} sudo ${binaryPath} ${args.join(' ')}`;
  }
  
  return new Promise((resolve, reject) => {
    const options = {
      name: '0E7 Desktop',
      icns: platform === 'darwin' ? path.join(__dirname, '..', 'build', 'icon.icns') : undefined
    };
    
    console.log('正在请求管理员权限启动后端...');
    sudo.exec(command, options, (error, stdout, stderr) => {
      if (error) {
        // 用户取消或权限提升失败
        if (error.message && error.message.includes('User did not grant permission')) {
          console.log('用户取消了权限提升请求');
        } else {
          console.warn('无法以管理员权限启动后端:', error.message);
        }
        reject(error);
      } else {
        console.log('后端已以管理员权限启动');
        if (stdout) console.log('输出:', stdout);
        if (stderr) console.warn('错误:', stderr);
        resolve();
      }
    });
  });
}

async function launchBackend(port) {
  const binaryPath = resolveBinaryPath();
  if (!fs.existsSync(binaryPath)) {
    throw new Error(`未找到 0E7 二进制文件: ${binaryPath}`);
  }
  
  // 使用用户目录的 .0e7/ 作为工作目录和配置文件路径
  const userDataDir = getUserDataDir();
  const configPath = resolveConfigPath();

  const args = ['--server', '--config', configPath, '--server-port', port.toString()];
  
  // 首先尝试检测是否有管理员权限
  const hasAdmin = await checkAdminPrivileges();
  
  if (!hasAdmin) {
    // 如果没有管理员权限，尝试提升权限
    console.log('检测到没有管理员权限，尝试以管理员权限启动后端...');
    try {
      await launchBackendWithElevation(port);
      // 如果成功以管理员权限启动，等待一下让进程启动
      await new Promise(resolve => setTimeout(resolve, 2000));
      // 注意：使用 sudo-prompt 启动的进程无法直接控制，所以这里只是尝试
      // 如果提升失败，会继续使用普通权限启动
      return;
    } catch (error) {
      console.warn('无法以管理员权限启动，使用普通权限启动:', error.message);
      // 继续使用普通权限启动
    }
  } else {
    console.log('当前已有管理员权限');
  }
  
  // 使用普通权限启动（或已有管理员权限）
  backendProcess = spawn(binaryPath, args, {
    env: {
      ...process.env,
      OE7_SERVER_PORT: port.toString()
    },
    cwd: userDataDir,  // 工作目录设置为用户目录的 .0e7/
    stdio: ['ignore', 'pipe', 'pipe'],
    windowsHide: true
  });

  attachLogging(backendProcess, '0E7');

  backendProcess.on('exit', (code, signal) => {
    if (code !== 0 && signal !== 'SIGTERM') {
      dialog.showErrorBox('0E7 已退出', `0E7 服务异常退出，退出码 ${code ?? '未知'}`);
      app.quit();
    }
  });
}

const debugPortPromise = findFreePort(35000, 44000)
  .then((port) => {
    app.commandLine.appendSwitch('remote-debugging-port', port.toString());
    return port;
  })
  .catch((err) => {
    console.warn('无法设置远程调试端口', err);
    return null;
  });

async function createWindow() {
  await debugPortPromise;
  const appPort = await findFreePort();

  await launchBackend(appPort);

  await waitOn({
    resources: [`http://127.0.0.1:${appPort}`],
    timeout: 60000,
    interval: 500,
    validateStatus: (status) => status >= 200 && status < 500
  });

  const mainWindow = new BrowserWindow({
    width: 1366,
    height: 900,
    minWidth: 1200,
    minHeight: 720,
    webPreferences: {
      contextIsolation: true,
      nodeIntegration: false,
      devTools: true  // 允许开发者工具
    }
  });

  mainWindow.loadURL(`http://127.0.0.1:${appPort}`);

  // 移除菜单栏，但保留开发者工具快捷键
  Menu.setApplicationMenu(null);

  // 注册快捷键打开开发者工具（F12 或 Cmd+Option+I / Ctrl+Shift+I）
  mainWindow.webContents.on('before-input-event', (event, input) => {
    // F12 键
    if (input.key === 'F12') {
      mainWindow.webContents.toggleDevTools();
      event.preventDefault();
    }
    // Cmd+Option+I (macOS) 或 Ctrl+Shift+I (Windows/Linux)
    if ((input.control || input.meta) && input.shift && input.key.toLowerCase() === 'i') {
      mainWindow.webContents.toggleDevTools();
      event.preventDefault();
    }
  });

  if (isDev) {
    mainWindow.webContents.openDevTools();
  }
}

function cleanupBackend() {
  if (backendProcess && !backendProcess.killed) {
    backendProcess.kill();
  }
  backendProcess = null;
}

app.whenReady().then(createWindow).catch((err) => {
  dialog.showErrorBox('启动失败', err?.message || '未知错误');
  console.error(err);
  app.quit();
});

app.on('window-all-closed', () => {
  cleanupBackend();
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('before-quit', cleanupBackend);
app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0) {
    createWindow().catch((err) => {
      dialog.showErrorBox('启动失败', err?.message || '未知错误');
      console.error(err);
    });
  }
});
process.on('exit', cleanupBackend);
process.on('SIGINT', () => {
  cleanupBackend();
  process.exit(0);
});
process.on('SIGTERM', () => {
  cleanupBackend();
  process.exit(0);
});

