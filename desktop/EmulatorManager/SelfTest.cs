using System.Net.Http;
using System.Runtime.InteropServices;

namespace EmulatorManager;

/// <summary>Headless self-test: exercises the REST client without any window.</summary>
public static class SelfTest
{
    [DllImport("kernel32.dll")]
    private static extern bool AttachConsole(int dwProcessId);

    public static int Run(string baseUrl, string token)
    {
        AttachConsole(-1);
        try
        {
            using var api = new Backend(baseUrl, token);
            var avds = api.Avds().GetAwaiter().GetResult();
            System.Console.WriteLine($"avds={avds.Count} running={string.Join(",", avds.Where(a => a.Running).Select(a => a.Name))}");
            var ver = api.Version().GetAwaiter().GetResult();
            System.Console.WriteLine($"version={ver.GetProperty("version").GetString()} api={ver.GetProperty("api_level").GetString()}");
            foreach (var a in avds.Where(a => a.Running).Take(1))
            {
                var png = api.Screenshot(a.Adb_port).GetAwaiter().GetResult();
                bool isPng = png.Length > 8 && png[0] == 0x89 && png[1] == (byte)'P';
                System.Console.WriteLine($"screenshot {a.Name}: {png.Length} bytes png={isPng}");
                if (!isPng) return 1;
            }
            System.Console.WriteLine("SELFTEST OK");
            return 0;
        }
        catch (HttpRequestException ex)
        {
            System.Console.WriteLine("SELFTEST FAIL: " + ex.Message);
            return 1;
        }
    }
}
