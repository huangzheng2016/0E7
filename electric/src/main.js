const { app, BrowserWindow, dialog } = require('electron');
const path = require('path');
const fs = require('fs');
const { spawn } = require('child_process');
const net = require('net');
const waitOn = require('wait-on');

let backendProcess = null;

const isDev = !app.isPackaged || process.env.NODE_ENV === 'development';

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
    throw new Error(`当前平台 ${process.platform} 暂不支持 Electric 打包`);
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

function resolveConfigPath(binaryPath) {
  return path.join(path.dirname(binaryPath), 'config.ini');
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

async function launchBackend(port) {
  const binaryPath = resolveBinaryPath();
  if (!fs.existsSync(binaryPath)) {
    throw new Error(`未找到 0E7 二进制文件: ${binaryPath}`);
  }
  const configPath = resolveConfigPath(binaryPath);

  const args = ['--server', '--config', configPath, '--server-port', port.toString()];
  backendProcess = spawn(binaryPath, args, {
    env: {
      ...process.env,
      OE7_SERVER_PORT: port.toString()
    },
    cwd: path.dirname(binaryPath),
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
      nodeIntegration: false
    }
  });

  mainWindow.loadURL(`http://127.0.0.1:${appPort}`);

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

