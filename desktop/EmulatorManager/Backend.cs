using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json;

namespace EmulatorManager;

/// <summary>Thin client for the emulator-cli REST API (see api/openapi.json).</summary>
public sealed class Backend : IDisposable
{
    private readonly HttpClient _http = new() { Timeout = TimeSpan.FromMinutes(6) };
    private static readonly JsonSerializerOptions JsonOpts = new() { PropertyNameCaseInsensitive = true };

    public string BaseUrl { get; }
    public string Token { get; }

    public Backend(string baseUrl, string token)
    {
        BaseUrl = baseUrl.TrimEnd('/');
        Token = token;
    }

    private HttpRequestMessage Req(HttpMethod m, string path)
    {
        var r = new HttpRequestMessage(m, BaseUrl + path);
        if (!string.IsNullOrEmpty(Token)) r.Headers.Authorization = new AuthenticationHeaderValue("Bearer", Token);
        return r;
    }

    private async Task<T> Send<T>(HttpMethod m, string path, object? body = null, string? contentType = null)
    {
        using var req = Req(m, path);
        if (body is not null)
        {
            req.Content = body is byte[] bytes
                ? new ByteArrayContent(bytes)
                : JsonContent.Create(body);
            if (contentType is not null) req.Content.Headers.ContentType = new MediaTypeHeaderValue(contentType);
        }
        using var res = await _http.SendAsync(req);
        var text = await res.Content.ReadAsStringAsync();
        if (!res.IsSuccessStatusCode) throw new HttpRequestException($"{(int)res.StatusCode} {text.Trim()}");
        if (typeof(T) == typeof(string)) return (T)(object)text;
        return JsonSerializer.Deserialize<T>(text, JsonOpts)
            ?? throw new InvalidOperationException("empty response");
    }

    public Task<List<AvdInfo>> Avds() => Send<List<AvdInfo>>(HttpMethod.Get, "/v1/avds");
    public Task<JsonElement> Version() => Send<JsonElement>(HttpMethod.Get, "/v1/version");
    public Task<JsonElement> StartAvd(string name, object opts) => Send<JsonElement>(HttpMethod.Post, $"/v1/avds/{name}/start", opts);
    public Task<JsonElement> StopAvd(string name) => Send<JsonElement>(HttpMethod.Post, $"/v1/avds/{name}/stop");
    public Task<List<string>> Snapshots(string name) => Send<List<string>>(HttpMethod.Get, $"/v1/avds/{name}/snapshots");
    public Task<JsonElement> SaveSnapshot(string name, string snap) =>
        Send<JsonElement>(HttpMethod.Post, $"/v1/avds/{name}/snapshots?name={Uri.EscapeDataString(snap)}");
    public Task<JsonElement> PatchConfig(string name, object patch) => Send<JsonElement>(HttpMethod.Put, $"/v1/avds/{name}/config", patch);
    public Task<string> Log(string name, int tail = 60) => Send<string>(HttpMethod.Get, $"/v1/avds/{name}/log?tail={tail}");
    public Task<JsonElement> InstallApk(int port, byte[] apk) =>
        Send<JsonElement>(HttpMethod.Post, $"/v1/apk/install?port={port}", apk, "application/vnd.android.package-archive");
    public Task<JsonElement> Input(int port, object input) =>
        Send<JsonElement>(HttpMethod.Post, $"/v1/adb/input?port={port}", input);
    public Task<JsonElement> WaitBoot(int port) => Send<JsonElement>(HttpMethod.Get, $"/v1/adb/wait?port={port}");

    public async Task<byte[]> Screenshot(int port, CancellationToken ct = default)
    {
        using var req = Req(HttpMethod.Get, $"/v1/screen.png?port={port}");
        using var res = await _http.SendAsync(req, ct);
        res.EnsureSuccessStatusCode();
        return await res.Content.ReadAsByteArrayAsync(ct);
    }

    public void Dispose() => _http.Dispose();
}

public sealed class AvdInfo
{
    public string Name { get; set; } = "";
    public string Target { get; set; } = "";
    public int Ram_mb { get; set; }
    public int Cpus { get; set; }
    public int Width { get; set; }
    public int Height { get; set; }
    public int Density { get; set; }
    public bool Running { get; set; }
    public string Serial { get; set; } = "";
    public int Adb_port { get; set; }
    public List<string> Snapshots { get; set; } = new();
}
