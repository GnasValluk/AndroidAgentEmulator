using System.IO;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media.Imaging;
using System.Windows.Threading;
using WinForms = System.Windows.Forms;

namespace EmulatorManager;

public partial class MainWindow : Window
{
    private Backend? _api;
    private List<AvdInfo> _avds = new();
    private readonly DispatcherTimer _previewTimer = new() { Interval = TimeSpan.FromSeconds(1) };
    private readonly WinForms.NotifyIcon _tray = new();
    private System.Windows.Point? _downAt;
    private System.Windows.Point _downPx;
    private int _frameW, _frameH;

    public MainWindow()
    {
        InitializeComponent();
        _previewTimer.Tick += async (_, _) => await RefreshPreview();
        SetupTray();
        Loaded += async (_, _) => await InitAsync();
        Closing += (_, _) => { _tray.Visible = false; };
        StateChanged += (_, _) =>
        {
            // Minimize to tray instead of taskbar.
            if (WindowState == WindowState.Minimized) { Hide(); _tray.ShowBalloonTip(2000, "Emulator Manager", "Running in tray", WinForms.ToolTipIcon.Info); }
        };
    }

    private void SetupTray()
    {
        _tray.Icon = System.Drawing.SystemIcons.Application;
        _tray.Text = "Emulator Manager";
        _tray.Visible = true;
        var menu = new WinForms.ContextMenuStrip();
        menu.Items.Add("Show", null, (_, _) => { Show(); WindowState = WindowState.Normal; Activate(); });
        menu.Items.Add("Exit", null, (_, _) => { _tray.Visible = false; System.Windows.Application.Current.Shutdown(); });
        _tray.ContextMenuStrip = menu;
        _tray.DoubleClick += (_, _) => { Show(); WindowState = WindowState.Normal; Activate(); };
    }

    private async Task InitAsync()
    {
        try
        {
            _api = new Backend(App.BaseUrl, App.Token);
            await RefreshAvds();
            StatusText.Text = "connected";
            _previewTimer.Start();
        }
        catch (Exception ex) { StatusText.Text = "backend error: " + ex.Message; }
    }

    private AvdInfo? Selected() => AvdList.SelectedItem as AvdInfo;

    private async Task RefreshAvds()
    {
        if (_api is null) return;
        _avds = await _api.Avds();
        AvdList.ItemsSource = null;
        AvdList.ItemsSource = _avds;
        if (AvdList.SelectedItem is null && _avds.Count > 0) AvdList.SelectedIndex = 0;
        SyncSelection();
    }

    private void SyncSelection()
    {
        var a = Selected();
        if (a is null) return;
        BootSnap.ItemsSource = a.Snapshots;
        CfgRam.Text = a.Ram_mb.ToString(); CfgCpu.Text = a.Cpus.ToString();
        CfgW.Text = a.Width.ToString(); CfgH.Text = a.Height.ToString(); CfgD.Text = a.Density.ToString();
        CfgMsg.Text = a.Running ? $"serial {a.Serial} · adb {a.Adb_port}" : "stopped";
        _ = LoadSnaps();
    }

    private async Task LoadSnaps()
    {
        var a = Selected();
        if (a is null || _api is null) return;
        try { SnapList.ItemsSource = await _api.Snapshots(a.Name); }
        catch (Exception ex) { SnapList.ItemsSource = new[] { "error: " + ex.Message }; }
    }

    private async void RefreshBtn_Click(object sender, RoutedEventArgs e) => await RefreshAvds();
    private void AvdList_SelectionChanged(object sender, SelectionChangedEventArgs e) => SyncSelection();

    private async void StartBtn_Click(object sender, RoutedEventArgs e)
    {
        var a = Selected();
        if (a is null || _api is null) return;
        try
        {
            StatusText.Text = $"starting {a.Name}…";
            await _api.StartAvd(a.Name, new
            {
                gpu = ((ComboBoxItem?)BootGpu.SelectedItem)?.Content?.ToString() ?? "host",
                snapshot = BootSnap.SelectedItem?.ToString() ?? "",
                no_save = true, no_window = true,
            });
            StatusText.Text = $"{a.Name} starting…";
            _ = WaitBootAsync(a.Name, a.Adb_port > 0 ? a.Adb_port : 5555);
        }
        catch (Exception ex) { StatusText.Text = "start failed: " + ex.Message; }
        await RefreshAvds();
    }

    private async Task WaitBootAsync(string name, int port)
    {
        try
        {
            if (_api is null) return;
            await _api.WaitBoot(port);
            _tray.ShowBalloonTip(3000, "Emulator Manager", $"{name} booted", WinForms.ToolTipIcon.Info);
            await RefreshAvds();
        }
        catch { /* wait timed out; user can retry */ }
    }

    private async void StopBtn_Click(object sender, RoutedEventArgs e)
    {
        var a = Selected();
        if (a is null || _api is null) return;
        try { await _api.StopAvd(a.Name); StatusText.Text = $"{a.Name} stopped"; }
        catch (Exception ex) { StatusText.Text = "stop failed: " + ex.Message; }
        await RefreshAvds();
    }

    // ---- embedded display: poll PNG, forward input ----

    private async Task RefreshPreview()
    {
        var a = Selected();
        if (a is null || _api is null || !a.Running || a.Adb_port <= 0) return;
        try
        {
            var png = await _api.Screenshot(a.Adb_port);
            using var ms = new MemoryStream(png);
            var bmp = new BitmapImage();
            bmp.BeginInit();
            bmp.CacheOption = BitmapCacheOption.OnLoad;
            bmp.StreamSource = ms;
            bmp.EndInit();
            bmp.Freeze();
            _frameW = bmp.PixelWidth; _frameH = bmp.PixelHeight;
            Screen.Source = bmp;
        }
        catch { /* keep last frame */ }
    }

    private (int x, int y)? ToGuest(System.Windows.Input.MouseEventArgs e)
    {
        if (_frameW <= 0 || Screen.ActualWidth <= 0) return null;
        var p = e.GetPosition(Screen);
        return ((int)(p.X / Screen.ActualWidth * _frameW), (int)(p.Y / Screen.ActualHeight * _frameH));
    }

    private void Screen_MouseDown(object sender, MouseButtonEventArgs e)
    {
        Screen.Focus();
        _downAt = e.GetPosition(Screen);
        var g = ToGuest(e);
        if (g is { } pt) _downPx = new System.Windows.Point(pt.x, pt.y);
        Screen.CaptureMouse();
    }

    private void Screen_MouseMove(object sender, System.Windows.Input.MouseEventArgs e) { /* swipe drawn on release */ }

    private async void Screen_MouseUp(object sender, MouseButtonEventArgs e)
    {
        Screen.ReleaseMouseCapture();
        if (_downAt is null) return;
        var up = e.GetPosition(Screen);
        var dx = up.X - _downAt.Value.X; var dy = up.Y - _downAt.Value.Y;
        _downAt = null;
        var a = Selected();
        if (a is null || _api is null || !a.Running) return;
        try
        {
            if (Math.Abs(dx) < 10 && Math.Abs(dy) < 10)
                await _api.Input(a.Adb_port, new { action = "tap", x = (int)_downPx.X, y = (int)_downPx.Y });
            else
            {
                var g = ToGuest(e) ?? ((int)_downPx.X, (int)_downPx.Y);
                await _api.Input(a.Adb_port, new { action = "swipe", x1 = (int)_downPx.X, y1 = (int)_downPx.Y, x2 = g.x, y2 = g.y, ms = 300 });
            }
        }
        catch { /* ignore transient input errors */ }
    }

    protected override async void OnKeyDown(System.Windows.Input.KeyEventArgs e)
    {
        base.OnKeyDown(e);
        var a = Selected();
        if (a is null || _api is null || !a.Running || !Screen.IsFocused) return;
        try
        {
            if (e.Key is >= Key.A and <= Key.Z or >= Key.D0 and <= Key.D9 or Key.Space)
            {
                var ch = e.Key switch
                {
                    Key.Space => " ",
                    >= Key.D0 and <= Key.D9 => ((char)('0' + (e.Key - Key.D0))).ToString(),
                    _ => e.Key.ToString().ToLower(),
                };
                await _api.Input(a.Adb_port, new { action = "text", text = ch });
                e.Handled = true;
            }
            else
            {
                int code = e.Key switch { Key.Enter => 66, Key.Back => 67, Key.Escape => 4, Key.Delete => 67, _ => 0 };
                if (code != 0) { await _api.Input(a.Adb_port, new { action = "key", code }); e.Handled = true; }
            }
        }
        catch { /* ignore */ }
    }

    private async void BackBtn_Click(object sender, RoutedEventArgs e) => await SendKey(4);
    private async void HomeBtn_Click(object sender, RoutedEventArgs e) => await SendKey(3);
    private async void RecentBtn_Click(object sender, RoutedEventArgs e) => await SendKey(187);
    private async Task SendKey(int code)
    {
        var a = Selected();
        if (a is null || _api is null || !a.Running) return;
        try { await _api.Input(a.Adb_port, new { action = "key", code }); } catch { }
    }

    private async void SendText_Click(object sender, RoutedEventArgs e)
    {
        var a = Selected();
        if (a is null || _api is null || !a.Running || string.IsNullOrEmpty(TypeBox.Text)) return;
        try { await _api.Input(a.Adb_port, new { action = "text", text = TypeBox.Text }); TypeBox.Clear(); }
        catch (Exception ex) { StatusText.Text = "send failed: " + ex.Message; }
    }

    private async void ShotBtn_Click(object sender, RoutedEventArgs e) => await RefreshPreview();

    private async void SaveSnap_Click(object sender, RoutedEventArgs e)
    {
        var a = Selected();
        if (a is null || _api is null || string.IsNullOrWhiteSpace(SnapName.Text)) return;
        try { await _api.SaveSnapshot(a.Name, SnapName.Text.Trim()); SnapName.Clear(); await LoadSnaps(); await RefreshAvds(); }
        catch (Exception ex) { StatusText.Text = "snapshot failed: " + ex.Message; }
    }

    private async void SnapReload_Click(object sender, RoutedEventArgs e) => await LoadSnaps();

    private async void ApplyCfg_Click(object sender, RoutedEventArgs e)
    {
        var a = Selected();
        if (a is null || _api is null) return;
        try
        {
            var r = await _api.PatchConfig(a.Name, new
            {
                ram_mb = int.Parse(CfgRam.Text), cpus = int.Parse(CfgCpu.Text),
                width = int.Parse(CfgW.Text), height = int.Parse(CfgH.Text), density = int.Parse(CfgD.Text),
            });
            CfgMsg.Text = "applied: " + r.ToString();
            await RefreshAvds();
        }
        catch (Exception ex) { CfgMsg.Text = "apply failed: " + ex.Message; }
    }

    private async void InstallBtn_Click(object sender, RoutedEventArgs e)
    {
        var a = Selected();
        if (a is null || _api is null || !a.Running) return;
        var dlg = new Microsoft.Win32.OpenFileDialog { Filter = "APK files (*.apk)|*.apk" };
        if (dlg.ShowDialog() != true) return;
        try
        {
            await _api.InstallApk(a.Adb_port, await File.ReadAllBytesAsync(dlg.FileName));
            StatusText.Text = "installed " + System.IO.Path.GetFileName(dlg.FileName);
        }
        catch (Exception ex) { StatusText.Text = "install failed: " + ex.Message; }
    }

    private async void LogReload_Click(object sender, RoutedEventArgs e)
    {
        var a = Selected();
        if (a is null || _api is null) return;
        try { LogBox.Text = await _api.Log(a.Name); }
        catch (Exception ex) { LogBox.Text = "error: " + ex.Message; }
    }
}
