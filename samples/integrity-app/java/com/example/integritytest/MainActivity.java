package com.example.integritytest;

import android.app.Activity;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.util.Base64;
import android.util.Log;
import android.widget.ScrollView;
import android.widget.TextView;

import com.google.android.play.core.integrity.IntegrityManager;
import com.google.android.play.core.integrity.IntegrityManagerFactory;
import com.google.android.play.core.integrity.IntegrityTokenRequest;
import com.google.android.play.core.integrity.IntegrityTokenResponse;
import com.google.android.gms.tasks.OnFailureListener;
import com.google.android.gms.tasks.OnSuccessListener;

import java.security.SecureRandom;

public class MainActivity extends Activity {
    private static final String TAG = "IntegrityTest";
    private TextView out;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        out = new TextView(this);
        out.setText("tap REQUEST to call Play Integrity API (classic, nonce, project=0)\n");
        out.setTextSize(16);
        out.setTextColor(0xFF000000);
        out.setPadding(32, 32, 32, 32);
        out.setBackgroundColor(0xFFFFFFFF);
        ScrollView scroll = new ScrollView(this);
        scroll.setBackgroundColor(0xFFFFFFFF);
        scroll.addView(out);
        setContentView(scroll);
        // Auto-run shortly after launch so no tapping is needed.
        new Handler(Looper.getMainLooper()).postDelayed(new Runnable() {
            @Override public void run() { requestToken(); }
        }, 1500);
    }

    private void append(final String s) {
        Log.i(TAG, s);
        runOnUiThread(new Runnable() {
            @Override public void run() { out.append(s + "\n"); }
        });
    }

    private void requestToken() {
        byte[] raw = new byte[32];
        new SecureRandom().nextBytes(raw);
        final String nonce = Base64.encodeToString(raw, Base64.URL_SAFE | Base64.NO_WRAP);
        append("nonce=" + nonce);
        try {
            IntegrityManager mgr = IntegrityManagerFactory.create(getApplicationContext());
            IntegrityTokenRequest req = IntegrityTokenRequest.builder()
                    .setCloudProjectNumber(0L)
                    .setNonce(nonce)
                    .build();
            long t0 = System.currentTimeMillis();
            mgr.requestIntegrityToken(req)
                    .addOnSuccessListener(new OnSuccessListener<IntegrityTokenResponse>() {
                        @Override public void onSuccess(IntegrityTokenResponse r) {
                            String token = r.token();
                            append("SUCCESS ms=" + (System.currentTimeMillis() - t0));
                            append("token_len=" + (token == null ? -1 : token.length()));
                            append("token_head=" + (token == null ? "null" : token.substring(0, Math.min(80, token.length()))));
                            append("NOTE: decrypt server-side with your Cloud project key to read verdicts.");
                        }
                    })
                    .addOnFailureListener(new OnFailureListener() {
                        @Override public void onFailure(Exception e) {
                            append("FAILURE ms=" + (System.currentTimeMillis() - t0));
                            append("error=" + e.getClass().getName() + ": " + e.getMessage());
                        }
                    });
        } catch (Exception e) {
            append("THROW: " + e.getClass().getName() + ": " + e.getMessage());
        }
    }
}
