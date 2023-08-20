<script  setup lang="ts">
import { Terminal, type IEvent } from 'xterm';
import 'xterm/css/xterm.css';
import { onMounted } from 'vue';
import { WebLinksAddon } from 'xterm-addon-web-links';
import { FitAddon } from 'xterm-addon-fit';

const baseTheme = {
    foreground: '#F8F8F8',
    background: '#2D2E2C',
    selection: '#5DA5D533',
    black: '#1E1E1D',
    brightBlack: '#262625',
    red: '#CE5C5C',
    brightRed: '#FF7272',
    green: '#5BCC5B',
    brightGreen: '#72FF72',
    yellow: '#CCCC5B',
    brightYellow: '#FFFF72',
    blue: '#5D5DD3',
    brightBlue: '#7279FF',
    magenta: '#BC5ED1',
    brightMagenta: '#E572FF',
    cyan: '#5DA5D5',
    brightCyan: '#72F0FF',
    white: '#F8F8F8',
    brightWhite: '#FFFFFF'
};

type chunk = {
    id: number,
    output: string,
    status: string
}

let terminal: Terminal;
function prompt(terminal: Terminal) {
    command = '';
    terminal.write('\n\r$ ');
}
var command = '';

var commands: {
    [key: string]: {
        f: () => void,
        description: string
    }
} = {
    help: {
        f: () => {
            showHelp()
        },
        description: 'Prints this help message'
    },
    list: {
        f: () => {
            get_list()
        },
        description: 'List recent Submissions'
    }
};

const fitAddon = new FitAddon();
onMounted(() => {
    const webLinksAddon = new WebLinksAddon();
    terminal = new Terminal({
        fontSize: 14,
        fontFamily: '"Cascadia Code", Menlo, monospace',
        theme: baseTheme,
        cursorBlink: true,
        allowProposedApi: true
    });
    terminal.loadAddon(webLinksAddon);
    terminal.loadAddon(fitAddon);
    terminal.open(document.getElementById('terminal')!);
    setTimeout(() => {
        fitAddon.fit();
    }, 100);
    terminal.writeln([
        '   \x1b[3m Remote Execution Tool\x1b[0m is a tool that can execute python code online!',
        '',
        ' ┌ \x1b[1mFeatures\x1b[0m ──────────────────────────────────────────────────────────────────┐',
        ' │                                                                            │',
        ' │  \x1b[31;1mUpload your zip                         \x1b[32mPerformance\x1b[0m                       │',
        ' │   RET enables you to pack the files       RET can respond to your input    │',
        ' │   and auto install the dependencies.      in the console box.\x1b[0m              │',
        ' │                                                                            │',
        ' │  \x1b[33;1mAccessible                                            \x1b[0m                    │',
        ' │   The process data is listed here.                                         │',
        ' │                                                                            │',
        ' └────────────────────────────────────────────────────────────────────────────┘',
        ''
    ].join('\n\r'));
    showHelp();
    document.querySelector('.xterm')?.addEventListener('wheel', e => {
        if (terminal.buffer.active.baseY > 0) {
            e.preventDefault();
        }
    });
    terminal.onData((data) => {
        switch (data) {
            case '\u0003': // Ctrl+C
                terminal.write('^C');
                prompt(terminal);
                break;
            case '\r': // Enter
                runCommand(terminal, command);
                command = '';
                break;
            case '\u007F': // Backspace (DEL)
                // Do not delete the prompt
                if (terminal.buffer.active.cursorX > 2) {
                    terminal.write('\b \b');
                    if (command.length > 0) {
                        command = command.substr(0, command.length - 1);
                    }
                }
                break;
            default: // Print all other characters for demo
                if (data >= String.fromCharCode(0x20) && data <= String.fromCharCode(0x7E) || data >= '\u00a0') {
                    command += data;
                    terminal.write(data);
                }
        }
    });

    const runCommand = (term: Terminal, text: string) => {
        const command = text.trim().split(' ')[0];
        if (command.length > 0) {
            term.writeln('');
            if (command in commands) {
                (commands[command].f as () => void)();
                return;
            }
            term.writeln(`${command}: command not found`);
        }
        prompt(term);
    }
    onTerminalResize()
});

const showHelp = () => {
    terminal.writeln([
        'Welcome to Remote Execution Tool! Try some of the commands below.',
        '',
        ...Object.keys(commands).map(e => `  ${e.padEnd(10)} ${commands[e].description}`)
    ].join('\n\r'));
    prompt(terminal);
}

const get_list = async () => {
    const res = await fetch('/webui/exploit_show_output', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        }
    }).then(res => {
        if (res.status != 200) {
            terminal.writeln(`\x1b[41;1m Error \x1b[0m \x1b[1mWe can't connect to the remote server!\x1b[0m`)
            return undefined
        }
        return res.json()
    })
    if (res !== undefined) {
        const array: Array<chunk> = res.result.reverse()
        array.forEach(element => {
            if (element.status == 'SUCCESS') {
                terminal.writeln(`\x1b[46;1m ID ${element.id} \x1b[0m\x1b[42;1m ${element.status} \x1b[0m \x1b[1mYour output:\x1b[0m\r`);
            } else {
                terminal.writeln(`\x1b[46;1m ID ${element.id} \x1b[0m\x1b\x1b[41;1m  ${element.status}  \x1b[0m \x1b[1mYour output:\x1b[0m\n\r`);
            }
            // terminal.writeln(`         \x1b[44;1m  OUTPUT \x1b[0m ${element.output}\n\r`);
            terminal.writeln(`${element.output}`);
        });
    }
    prompt(terminal);
}

// const debounce=(fn:()=>void,wait:number|undefined)=>{
//     let timer:NodeJS.Timeout | null=null;
//     return function(){
//         if(timer !== null){
//             clearTimeout(timer);
//         }
//         timer = setTimeout(fn,wait);
//     }
// }

const onResize = () =>
//debounce(
{
    fitAddon.fit()
    console.log(1)
}
//, 500)

const onTerminalResize = () => {
    window.addEventListener('resize', onResize)
}
</script>
<template>
    <div id="console-wrapper">
        <div id="terminal"></div>
    </div>
</template>

<style scoped>
#terminal {
    width: 100%;
    height: 100%;
    margin-top: 20px;
}

#console-wrapper {
    width: 100%;
    padding: 1px 20px 20px 20px;
    border-radius: 5px;
    background-color: #2D2E2C;
}
</style>

<style>
.xterm ::-webkit-scrollbar {
    width: 0 !important
}

.xterm {
    overflow: -moz-scrollbars-none;
}

.xterm {
    -ms-overflow-style: none;
}

textarea {
    overflow: hidden;
}
</style>