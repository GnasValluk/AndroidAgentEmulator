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
            // Run off the UI thread: blocking on async code here would deadlock
            // against the WPF dispatcher synchronization context.
            int code = Task.Run(() => SelfTest.Run(BaseUrl, ResolveToken())).GetAwaiter().GetResult();
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
        File.WriteAllText(f, token, new System.Text.UTF8Encoding(false));
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
        if (IsUp() && BackendHealthy())
            return;
        // A stale serve (old version, wrong token/env) is holding the port.
        // Only our own backend binary is ever killed here.
        foreach (var p in Process.GetProcessesByName("emulator-cli"))
        {
            try { p.Kill(); } catch { /* best effort */ }
        }
        Thread.Sleep(1500);
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
        foreach (var (key, value) in DiscoverSdkEnv())
            psi.Environment[key] = value;
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

    internal static bool BackendHealthy()
    {
        try
        {
            using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(5) };
            var req = new HttpRequestMessage(HttpMethod.Get, BaseUrl + "/v1/avds");
            req.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", Token);
            using var res = http.Send(req);
            return res.IsSuccessStatusCode;
        }
        catch { return false; }
    }

    /// <summary>
    /// Locate SDK + AVD dirs: explicit env first, then well-known locations.
    /// Always set for the backend child so the installed app works without
    /// any user-side environment configuration.
    /// </summary>
    internal static Dictionary<string, string> DiscoverSdkEnv()
    {
        var out_ = new Dictionary<string, string>();
        string? SdkAt(string d) =>
            File.Exists(Path.Combine(d, "emulator", "emulator.exe")) ? d : null;
        var sdk = Environment.GetEnvironmentVariable("ANDROID_SDK_ROOT")
            ?? Environment.GetEnvironmentVariable("EMULATOR_SDK_ROOT")
            ?? SdkAt(@"D:\data\android-sdk")
            ?? SdkAt(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), @"Android\Sdk"))
            ?? SdkAt(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), @"Android\Sdk"));
        if (sdk is not null) out_["ANDROID_SDK_ROOT"] = sdk;
        bool AvdAt(string d) => Directory.Exists(d);
        var avd = Environment.GetEnvironmentVariable("ANDROID_AVD_HOME")
            ?? Environment.GetEnvironmentVariable("EMULATOR_AVD_HOME");
        if (avd is null)
        {
            foreach (var cand in new[]
            {
                @"D:\data\avd",
                Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), @".android\avd"),
            })
                if (AvdAt(cand)) { avd = cand; break; }
        }
        if (avd is not null) out_["ANDROID_AVD_HOME"] = avd;
        return out_;
    }
}
