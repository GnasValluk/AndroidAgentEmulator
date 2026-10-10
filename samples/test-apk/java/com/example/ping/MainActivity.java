package com.example.ping;

import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;

public class MainActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        TextView tv = new TextView(this);
        tv.setText("ping from AndroidAgentEmulator");
        tv.setTextSize(24);
        tv.setTextColor(0xFF000000);
        tv.setBackgroundColor(0xFFFFFFFF);
        tv.setPadding(48, 200, 48, 48);
        setContentView(tv);
    }
}
