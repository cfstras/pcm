function PCMTerm(argv) {
    this.argv = argv;
    window.term = this;
};
PCMTerm.init = function() {

    // If you are a cross-browser web app and want to use window.localStorage
    //hterm.defaultStorage = new lib.Storage.Local()

    // If you are a cross-browser web app and want in-memory storage only
    hterm.defaultStorage = new lib.Storage.Memory();

    // opt_profileName is the name of the terminal profile to load, or "default" if
    // not specified.  If you're using one of the persistent storage
    // implementations then this will scope all preferences read/writes to this
    // name.
    var terminal = new hterm.Terminal("default");
    terminal.decorate(document.querySelector('#terminal'));

    terminal.onTerminalReady = function () {

        terminal.keyboard.bindings.addBinding('Ctrl-Shift-P', function () {
            console.log("special keybinding Ctrl-Shift-P!", arguments);
            return hterm.Keyboard.KeyActions.CANCEL;
        });
        terminal.setCursorPosition(0, 0);
        terminal.setCursorVisible(true);
        terminal.runCommandClass(PCMTerm, document.location.hash.substr(1));
        terminal.command.keyboard_ = terminal.keyboard;
    };
};

PCMTerm.prototype.run = function(argv) {
    this.io = this.argv.io.push();

    this.socket = new WebSocket(
        location.origin.replace("http", "ws") + "/socket/");
    this.socket.binaryType = "blob";
    this.socket.onmessage = function(ev) {
        console.log("ssh:", ev.data)
        if (!ev.data) {
            return;
        }
        switch(ev.data[0]) {
            case 'D':
                this.io.print(ev.data.slice(1));
                break;
            default:
                console.warn("unknown msg:", ev)
        }
    }.bind(this);

    // Create a new terminal IO object and give it the foreground.
    // (The default IO object just prints warning messages about unhandled
    // things to the the JS console.)
    //var io = terminal.io.push();

    this.io.onVTKeystroke = this.input.bind(this, false);
    this.io.sendString = this.input.bind(this, true);

    this.io.onTerminalResize = function (columns, rows) {
        // React to size changes here.
        // Secure Shell pokes at NaCl, which eventually results in
        // some ioctls on the host.
        console.log("resize", columns, rows);
        if (!this.socket) {return;}
        this.socket.send("J"+JSON.stringify({
                resize: { cols: columns, rows: rows } }));
    }.bind(this);
    // You can call io.push() to foreground a fresh io context, which can
    // be uses to give control of the terminal to something else.  When that
    // thing is complete, should call io.pop() to restore control to the
    // previous io object.
    //terminal.installKeyboard();

    this.io.println('Loading pcm...');
};

PCMTerm.prototype.input = function(fromTerm, str) {
    console.log(fromTerm, str);
    this.socket.send(new Blob([str]));
};
