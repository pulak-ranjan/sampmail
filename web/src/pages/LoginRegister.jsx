import React, { useState, useEffect } from "react";
import { useAuth } from "../AuthContext";
import { useNavigate } from "react-router-dom";
import { Mail, Lock, Key, ArrowRight, Loader2, ShieldCheck, UserPlus, LogIn, CheckCircle2 } from "lucide-react";
import { cn } from "../lib/utils";

export default function LoginRegister() {
  const { login, verify2FA, register, user } = useAuth();
  const [mode, setMode] = useState("login"); // login vs register
  const [step, setStep] = useState(1); // 1 = creds, 2 = 2fa

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [totp, setTotp] = useState("");
  const [tempToken, setTempToken] = useState("");

  const [error, setError] = useState("");
  const [successMsg, setSuccessMsg] = useState("");
  const [busy, setBusy] = useState(false);

  const navigate = useNavigate();

  useEffect(() => {
    if (user) {
      navigate("/");
    }
  }, [user, navigate]);

  const onSubmit = async (e) => {
    e.preventDefault();
    setError("");
    setSuccessMsg("");
    setBusy(true);

    try {
      if (mode === "register") {
        await register(email, password);
        setSuccessMsg("Account created successfully! Please login.");
        setMode("login"); // Auto-switch to login
        setPassword(""); // Clear password for security
      } else {
        if (step === 1) {
          const res = await login(email, password);
          if (res && res.requires_2fa) {
            setTempToken(res.temp_token);
            setStep(2);
            setError("");
          } else {
            navigate("/");
          }
        } else {
          await verify2FA(tempToken, totp);
          navigate("/");
        }
      }
    } catch (err) {
      setError(err.message || "Authentication failed");
    } finally {
      setBusy(false);
    }
  };

  const toggleMode = () => {
    setMode(mode === "login" ? "register" : "login");
    setError("");
    setSuccessMsg("");
  };

  return (
    <div className="min-h-screen w-full flex items-center justify-center p-4 bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-slate-900 via-slate-950 to-black text-slate-100 relative overflow-hidden">
      {/* Background Ambience */}
      <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-blue-500/10 rounded-full blur-[120px] pointer-events-none" />
      <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-purple-500/10 rounded-full blur-[120px] pointer-events-none" />

      <div className="w-full max-w-md relative z-10 backdrop-blur-xl bg-slate-900/50 border border-slate-800/50 rounded-2xl shadow-2xl p-8 transition-all duration-500 hover:border-slate-700/50">

        {/* Header */}
        <div className="text-center space-y-3 mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-br from-blue-500/20 to-purple-500/20 text-blue-400 mb-2 shadow-inner ring-1 ring-white/10">
            {step === 2 ? <ShieldCheck className="w-8 h-8" /> : <Lock className="w-8 h-8" />}
          </div>
          <h1 className="text-3xl font-bold tracking-tight bg-gradient-to-r from-white to-slate-400 bg-clip-text text-transparent">
            {step === 2 ? "Two-Factor Auth" : mode === "login" ? "Welcome Back" : "Get Started"}
          </h1>
          <p className="text-sm text-slate-400">
            {step === 2
              ? "Check your authenticator app"
              : mode === "login"
                ? "Enter your credentials to access the console"
                : "Create your admin account to begin"}
          </p>
        </div>

        {/* Notifications */}
        {successMsg && (
          <div className="mb-6 p-4 rounded-lg bg-green-500/10 border border-green-500/20 text-green-400 text-sm font-medium flex items-center gap-2 animate-in fade-in slide-in-from-top-2">
            <CheckCircle2 className="w-4 h-4" />
            {successMsg}
          </div>
        )}

        {error && (
          <div className="mb-6 p-4 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm font-medium text-center animate-in fade-in slide-in-from-top-2">
            {error}
          </div>
        )}

        {/* Form */}
        <form onSubmit={onSubmit} className="space-y-5">
          {step === 1 ? (
            <div className="space-y-4">
              <div className="relative group">
                <Mail className="absolute left-3 top-3 h-5 w-5 text-slate-500 group-focus-within:text-blue-400 transition-colors" />
                <input
                  type="email"
                  placeholder="name@company.com"
                  className="w-full bg-slate-950/50 border border-slate-800 rounded-lg py-3 pl-11 pr-4 text-sm text-slate-100 placeholder:text-slate-600 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500/50 transition-all shadow-sm"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  autoFocus
                />
              </div>
              <div className="relative group">
                <Key className="absolute left-3 top-3 h-5 w-5 text-slate-500 group-focus-within:text-blue-400 transition-colors" />
                <input
                  type="password"
                  placeholder="Password"
                  className="w-full bg-slate-950/50 border border-slate-800 rounded-lg py-3 pl-11 pr-4 text-sm text-slate-100 placeholder:text-slate-600 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500/50 transition-all shadow-sm"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              <input
                type="text"
                className="w-full h-14 text-center text-3xl tracking-[0.5em] font-mono rounded-lg border border-slate-800 bg-slate-950/50 text-white placeholder:text-slate-700/50 focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500/50 transition-all"
                value={totp}
                onChange={(e) => setTotp(e.target.value)}
                placeholder="000000"
                maxLength={6}
                required
                autoFocus
              />
            </div>
          )}

          <button
            type="submit"
            disabled={busy}
            className="w-full bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 text-white font-medium py-3 rounded-lg shadow-lg shadow-blue-500/20 active:scale-95 transition-all duration-200 flex items-center justify-center disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {busy ? (
              <Loader2 className="mr-2 h-5 w-5 animate-spin" />
            ) : step === 2 ? (
              "Verify Code"
            ) : mode === "register" ? (
              <span className="flex items-center">Create Account <ArrowRight className="ml-2 h-4 w-4" /></span>
            ) : (
              <span className="flex items-center">Sign In <ArrowRight className="ml-2 h-4 w-4" /></span>
            )}
          </button>
        </form>

        {/* Footer / Toggle */}
        {step === 1 && (
          <div className="mt-8 text-center">
            <p className="text-sm text-slate-400">
              {mode === "login" ? "Don't have an account yet?" : "Already have an account?"}
              <button
                type="button"
                onClick={toggleMode}
                className="ml-2 font-medium text-blue-400 hover:text-blue-300 transition-colors focus:outline-none hover:underline"
              >
                {mode === "login" ? "Register now" : "Log in"}
              </button>
            </p>
          </div>
        )}

        {step === 2 && (
          <div className="mt-6 text-center">
            <button
              onClick={() => { setStep(1); setPassword(""); setTempToken(""); }}
              className="text-sm text-slate-500 hover:text-slate-300 transition-colors"
            >
              ← Back to Login
            </button>
          </div>
        )}
      </div>

      {/* Branding Footer */}
      <div className="absolute bottom-6 text-xs text-slate-600 font-medium tracking-wide pointer-events-none">
        POWERED BY
        <span className="text-slate-500 ml-1">SAMPMAIL</span>
      </div>
    </div>
  );
}
