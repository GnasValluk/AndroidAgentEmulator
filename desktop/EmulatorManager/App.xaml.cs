using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Threading;

namespace EmulatorManager;

public partial class App : System.Windows.Application
{
    public const string BaseUrl = "http://127.0.0.1:18037";
    public static string Token { get; private set; } = "";
    public static string CliPath { get; private set; } = "";
    private static Mutex? _mutex;
    private static Process? _serveProc;

    protected override void OnStartup(System.Windows.StartupEventArgs e)
    {
        base.OnStartup(e);
        if (e.Args.Contains("--self-test"))
        {
            int code = SelfTest.Run(BaseUrl, ResolveToken());
            Shutdown(code);
            return;
        }
        _mutex = new Mutex(true, "EmulatorManagerSingleInstance", out bool fresh);
        if (!fresh) { Shutdown(0); return; }
        Token = ResolveToken();
        CliPath = ResolveCli();
        EnsureBackend();
    }

    protected override void OnExit(System.Windows.ExitEventArgs e)
    {
        try
        {
            // Stop only the serve process we spawned.
            if (_serveProc is { HasExited: false }) _serveProc.Kill();
        }
        catch { /* best effort */ }
        _mutex?.ReleaseMutex();
        base.OnExit(e);
    }

    internal static string AppDataDir()
    {
        var d = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "EmulatorManager");
        Directory.CreateDirectory(d);
        return d;
    }

    internal static string ResolveToken()
    {
        var env = Environment.GetEnvironmentVariable("EMULATOR_API_TOKEN");
        if (!string.IsNullOrWhiteSpace(env)) return env.Trim();
        var f = Path.Combine(AppDataDir(), "token.txt");
        if (File.Exists(f)) return File.ReadAllText(f).Trim();
        var token = Convert.ToHexString(System.Security.Cryptography.RandomNumberGenerator.GetBytes(16));
        File.WriteAllText(f, token);
        return token;
    }

    internal static string ResolveCli()
    {
        var local = Path.Combine(AppContext.BaseDirectory, "emulator-cli.exe");
        if (File.Exists(local)) return local;
        var path = Environment.GetEnvironmentVariable("PATH") ?? "";
        foreach (var dir in path.Split(';'))
        {
            try
            {
                var cand = Path.Combine(dir.Trim(), "emulator-cli.exe");
                if (File.Exists(cand)) return cand;
            }
            catch { /* ignore bad entries */ }
        }
        return "emulator-cli.exe";
    }

    internal static void EnsureBackend()
    {
        if (IsUp()) return;
        var psi = new ProcessStartInfo
        {
            FileName = CliPath,
            Arguments = "--api 16 serve",
            UseShellExecute = false,
            CreateNoWindow = true,
            WorkingDirectory = AppContext.BaseDirectory,
        };
        psi.Environment["EMULATOR_LISTEN_ADDR"] = "127.0.0.1:18037";
        psi.Environment["EMULATOR_API_TOKEN"] = Token;
        foreach (var k in new[] { "ANDROID_SDK_ROOT", "ANDROID_AVD_HOME" })
        {
            var v = Environment.GetEnvironmentVariable(k);
            if (!string.IsNullOrEmpty(v)) psi.Environment[k] = v;
        }
        _serveProc = Process.Start(psi);
        var deadline = DateTime.UtcNow.AddSeconds(45);
        while (DateTime.UtcNow < deadline)
        {
            if (IsUp()) return;
            Thread.Sleep(1000);
        }
        throw new InvalidOperationException("emulator-cli serve did not come up. Is another copy running?");
    }

    internal static bool IsUp()
    {
        try
        {
            using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(3) };
            using var res = http.GetAsync(BaseUrl + "/v1/version").GetAwaiter().GetResult();
            return res.StatusCode == System.Net.HttpStatusCode.OK ||
                   res.StatusCode == System.Net.HttpStatusCode.Unauthorized;
        }
        catch { return false; }
    }
}
