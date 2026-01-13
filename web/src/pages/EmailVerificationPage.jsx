import React, { useState } from "react";
import { Mail, Search, Upload, CheckCircle, XCircle, AlertCircle, Loader2, RefreshCw, Download, FileText } from "lucide-react";
import { cn } from "../lib/utils";

const API_BASE = "/api";

async function apiRequest(endpoint, options = {}) {
    const token = localStorage.getItem("sampmail_token");
    const res = await fetch(`${API_BASE}${endpoint}`, {
        ...options,
        headers: {
            "Content-Type": "application/json",
            Authorization: token ? `Bearer ${token}` : "",
            ...options.headers,
        },
        body: options.body ? JSON.stringify(options.body) : undefined,
    });
    if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.error || "Request failed");
    }
    return res.json();
}

export default function EmailVerificationPage() {
    const [activeTab, setActiveTab] = useState("single");

    // Single verification
    const [singleEmail, setSingleEmail] = useState("");
    const [singleResult, setSingleResult] = useState(null);
    const [singleLoading, setSingleLoading] = useState(false);

    // Bulk verification
    const [bulkEmails, setBulkEmails] = useState("");
    const [bulkResults, setBulkResults] = useState([]);
    const [bulkLoading, setBulkLoading] = useState(false);
    const [bulkProgress, setBulkProgress] = useState(0);

    const [error, setError] = useState(null);

    const verifySingle = async (e) => {
        e.preventDefault();
        if (!singleEmail.trim()) return;

        setSingleLoading(true);
        setSingleResult(null);
        setError(null);

        try {
            const result = await apiRequest("/contacts/verify", {
                method: "POST",
                body: { email: singleEmail.trim() }
            });
            setSingleResult(result);
        } catch (err) {
            setError(err.message);
        } finally {
            setSingleLoading(false);
        }
    };

    const verifyBulk = async (e) => {
        e.preventDefault();
        const emails = bulkEmails.split('\n').map(e => e.trim()).filter(e => e && e.includes('@'));
        if (emails.length === 0) {
            setError("Please enter at least one valid email");
            return;
        }

        setBulkLoading(true);
        setBulkResults([]);
        setBulkProgress(0);
        setError(null);

        try {
            const result = await apiRequest("/contacts/verify-batch", {
                method: "POST",
                body: { emails }
            });
            setBulkResults(result.results || []);
            setBulkProgress(100);
        } catch (err) {
            setError(err.message);
        } finally {
            setBulkLoading(false);
        }
    };

    const exportResults = () => {
        const csv = ["email,status,score,mx_valid,disposable,log"];
        bulkResults.forEach(r => {
            csv.push(`${r.email},${r.status},${r.score || ''},${r.mx_valid || ''},${r.disposable || ''},${(r.log || '').replace(/,/g, ';')}`);
        });
        const blob = new Blob([csv.join('\n')], { type: 'text/csv' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'verification_results.csv';
        a.click();
    };

    const getStatusIcon = (status) => {
        switch (status) {
            case 'valid':
            case 'deliverable':
                return <CheckCircle className="w-5 h-5 text-green-500" />;
            case 'invalid':
            case 'undeliverable':
                return <XCircle className="w-5 h-5 text-red-500" />;
            case 'risky':
            case 'unknown':
                return <AlertCircle className="w-5 h-5 text-yellow-500" />;
            default:
                return <AlertCircle className="w-5 h-5 text-muted-foreground" />;
        }
    };

    const getStatusColor = (status) => {
        switch (status) {
            case 'valid':
            case 'deliverable':
                return 'bg-green-500/10 text-green-500 border-green-500/30';
            case 'invalid':
            case 'undeliverable':
                return 'bg-red-500/10 text-red-500 border-red-500/30';
            case 'risky':
            case 'unknown':
                return 'bg-yellow-500/10 text-yellow-500 border-yellow-500/30';
            default:
                return 'bg-muted text-muted-foreground border-border';
        }
    };

    return (
        <div className="space-y-6">
            <div>
                <h1 className="text-3xl font-bold text-foreground">Email Verification</h1>
                <p className="text-muted-foreground">Verify email addresses before sending to improve deliverability</p>
            </div>

            {error && (
                <div className="p-4 bg-red-500/10 text-red-400 rounded-lg border border-red-500/20 flex items-center justify-between">
                    {error}
                    <button onClick={() => setError(null)}><XCircle size={16} /></button>
                </div>
            )}

            {/* Tabs */}
            <div className="flex space-x-2 border-b border-border">
                <button
                    onClick={() => setActiveTab("single")}
                    className={cn(
                        "px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px",
                        activeTab === "single"
                            ? "border-primary text-primary"
                            : "border-transparent text-muted-foreground hover:text-foreground"
                    )}
                >
                    <Mail className="w-4 h-4 inline mr-2" />
                    Single Verification
                </button>
                <button
                    onClick={() => setActiveTab("bulk")}
                    className={cn(
                        "px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px",
                        activeTab === "bulk"
                            ? "border-primary text-primary"
                            : "border-transparent text-muted-foreground hover:text-foreground"
                    )}
                >
                    <Upload className="w-4 h-4 inline mr-2" />
                    Bulk Verification
                </button>
            </div>

            {/* Single Verification Tab */}
            {activeTab === "single" && (
                <div className="grid lg:grid-cols-2 gap-6">
                    <div className="bg-card border border-border rounded-xl p-6">
                        <h3 className="font-semibold mb-4 flex items-center gap-2">
                            <Search className="w-5 h-5" />
                            Verify Email Address
                        </h3>
                        <form onSubmit={verifySingle} className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-muted-foreground mb-2">Email Address</label>
                                <div className="flex gap-2">
                                    <input
                                        type="email"
                                        value={singleEmail}
                                        onChange={(e) => setSingleEmail(e.target.value)}
                                        placeholder="user@example.com"
                                        className="flex-1 bg-background border border-border rounded-lg p-3 text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                                        required
                                    />
                                    <button
                                        type="submit"
                                        disabled={singleLoading}
                                        className="px-6 py-3 bg-primary text-primary-foreground rounded-lg font-medium hover:bg-primary/90 transition-colors flex items-center gap-2 disabled:opacity-50"
                                    >
                                        {singleLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Search className="w-4 h-4" />}
                                        Verify
                                    </button>
                                </div>
                            </div>
                        </form>

                        <div className="mt-6 text-sm text-muted-foreground">
                            <h4 className="font-medium mb-2 text-foreground">What we check:</h4>
                            <ul className="space-y-1">
                                <li>✓ Email format validation</li>
                                <li>✓ MX record lookup</li>
                                <li>✓ SMTP verification</li>
                                <li>✓ Disposable email detection</li>
                                <li>✓ Catch-all detection</li>
                            </ul>
                        </div>
                    </div>

                    {/* Single Result */}
                    <div className="bg-card border border-border rounded-xl p-6">
                        <h3 className="font-semibold mb-4">Verification Result</h3>
                        {singleResult ? (
                            <div className="space-y-4">
                                <div className="flex items-center gap-3">
                                    {getStatusIcon(singleResult.status)}
                                    <div>
                                        <div className="font-medium">{singleResult.email}</div>
                                        <span className={cn("px-2 py-0.5 rounded text-xs border", getStatusColor(singleResult.status))}>
                                            {singleResult.status?.toUpperCase()}
                                        </span>
                                    </div>
                                </div>

                                <div className="grid grid-cols-2 gap-4 text-sm">
                                    <div className="bg-muted/30 rounded-lg p-3">
                                        <div className="text-muted-foreground">Score</div>
                                        <div className="font-semibold text-lg">{singleResult.risk_score ?? singleResult.score ?? 'N/A'}</div>
                                    </div>
                                    <div className="bg-muted/30 rounded-lg p-3">
                                        <div className="text-muted-foreground">MX Valid</div>
                                        <div className="font-semibold text-lg">{(singleResult.mx?.accepts_mail || singleResult.mx_valid) ? 'Yes' : 'No'}</div>
                                    </div>
                                    <div className="bg-muted/30 rounded-lg p-3">
                                        <div className="text-muted-foreground">Disposable</div>
                                        <div className="font-semibold text-lg">{(singleResult.misc?.is_disposable || singleResult.disposable) ? 'Yes' : 'No'}</div>
                                    </div>
                                    <div className="bg-muted/30 rounded-lg p-3">
                                        <div className="text-muted-foreground">Catch-All</div>
                                        <div className="font-semibold text-lg">{(singleResult.smtp?.is_catch_all || singleResult.catch_all) ? 'Yes' : 'No'}</div>
                                    </div>
                                    <div className="bg-muted/30 rounded-lg p-3">
                                        <div className="text-muted-foreground">Deliverable</div>
                                        <div className="font-semibold text-lg">{(singleResult.smtp?.is_deliverable || singleResult.is_reachable === 'safe') ? 'Yes' : 'No'}</div>
                                    </div>
                                    <div className="bg-muted/30 rounded-lg p-3">
                                        <div className="text-muted-foreground">Status</div>
                                        <div className="font-semibold text-lg capitalize">{singleResult.is_reachable || singleResult.status || 'Unknown'}</div>
                                    </div>
                                </div>

                                {singleResult.log && (
                                    <div className="bg-muted/30 rounded-lg p-3">
                                        <div className="text-muted-foreground text-sm mb-1">Verification Log</div>
                                        <pre className="text-xs font-mono whitespace-pre-wrap">{singleResult.log}</pre>
                                    </div>
                                )}
                            </div>
                        ) : (
                            <div className="text-center py-12 text-muted-foreground">
                                <Mail className="w-12 h-12 mx-auto mb-3 opacity-50" />
                                <p>Enter an email address to verify</p>
                            </div>
                        )}
                    </div>
                </div>
            )}

            {/* Bulk Verification Tab */}
            {activeTab === "bulk" && (
                <div className="space-y-6">
                    <div className="grid lg:grid-cols-3 gap-6">
                        <div className="lg:col-span-1 bg-card border border-border rounded-xl p-6">
                            <h3 className="font-semibold mb-4 flex items-center gap-2">
                                <FileText className="w-5 h-5" />
                                Enter Emails
                            </h3>

                            {/* CSV Upload Section */}
                            <div className="mb-4">
                                <label
                                    htmlFor="csv-upload"
                                    className="flex flex-col items-center justify-center w-full h-24 border-2 border-dashed border-border rounded-lg cursor-pointer hover:border-primary/50 transition-colors bg-muted/30"
                                >
                                    <div className="flex flex-col items-center justify-center py-2">
                                        <Upload className="w-6 h-6 mb-1 text-muted-foreground" />
                                        <p className="text-sm text-muted-foreground">
                                            <span className="font-medium text-primary">Upload CSV</span> or drag & drop
                                        </p>
                                        <p className="text-xs text-muted-foreground">First column = emails</p>
                                    </div>
                                    <input
                                        id="csv-upload"
                                        type="file"
                                        accept=".csv,.txt"
                                        className="hidden"
                                        onChange={(e) => {
                                            const file = e.target.files?.[0];
                                            if (!file) return;
                                            const reader = new FileReader();
                                            reader.onload = (event) => {
                                                const text = event.target?.result;
                                                if (typeof text === 'string') {
                                                    // Parse CSV - extract emails from first column
                                                    const lines = text.split(/[\r\n]+/).filter(l => l.trim());
                                                    const emails = lines.map(line => {
                                                        const firstCol = line.split(',')[0].replace(/"/g, '').trim();
                                                        return firstCol.includes('@') ? firstCol : '';
                                                    }).filter(e => e);
                                                    setBulkEmails(emails.join('\n'));
                                                }
                                            };
                                            reader.readAsText(file);
                                            e.target.value = ''; // Reset for re-upload
                                        }}
                                    />
                                </label>
                            </div>

                            <div className="relative mb-3">
                                <div className="absolute inset-0 flex items-center">
                                    <span className="w-full border-t" />
                                </div>
                                <div className="relative flex justify-center text-xs uppercase">
                                    <span className="bg-card px-2 text-muted-foreground">or paste manually</span>
                                </div>
                            </div>

                            <form onSubmit={verifyBulk} className="space-y-4">
                                <textarea
                                    value={bulkEmails}
                                    onChange={(e) => setBulkEmails(e.target.value)}
                                    rows={8}
                                    placeholder={"user1@example.com\nuser2@example.com\nuser3@example.com"}
                                    className="w-full bg-background border border-border rounded-lg p-3 text-foreground text-sm font-mono focus:ring-2 focus:ring-primary focus:outline-none resize-none"
                                />
                                <div className="flex justify-between text-sm text-muted-foreground">
                                    <span>{bulkEmails.split('\n').filter(e => e.trim() && e.includes('@')).length} emails</span>
                                    {bulkEmails && (
                                        <button type="button" onClick={() => setBulkEmails('')} className="text-red-400 hover:text-red-300">
                                            Clear
                                        </button>
                                    )}
                                </div>
                                <button
                                    type="submit"
                                    disabled={bulkLoading}
                                    className="w-full px-6 py-3 bg-primary text-primary-foreground rounded-lg font-medium hover:bg-primary/90 transition-colors flex items-center justify-center gap-2 disabled:opacity-50"
                                >
                                    {bulkLoading ? (
                                        <>
                                            <Loader2 className="w-4 h-4 animate-spin" />
                                            Verifying... {bulkProgress}%
                                        </>
                                    ) : (
                                        <>
                                            <RefreshCw className="w-4 h-4" />
                                            Verify All
                                        </>
                                    )}
                                </button>
                            </form>
                        </div>

                        <div className="lg:col-span-2 bg-card border border-border rounded-xl p-6">
                            <div className="flex items-center justify-between mb-4">
                                <h3 className="font-semibold">Results ({bulkResults.length})</h3>
                                {bulkResults.length > 0 && (
                                    <button
                                        onClick={exportResults}
                                        className="px-3 py-1.5 text-sm bg-muted hover:bg-muted/80 rounded-lg flex items-center gap-2"
                                    >
                                        <Download className="w-4 h-4" />
                                        Export CSV
                                    </button>
                                )}
                            </div>

                            {bulkResults.length > 0 ? (
                                <div className="max-h-[500px] overflow-y-auto space-y-2">
                                    {bulkResults.map((r, i) => (
                                        <div key={i} className="flex items-center justify-between p-3 bg-muted/30 rounded-lg">
                                            <div className="flex items-center gap-3">
                                                {getStatusIcon(r.status)}
                                                <span className="font-mono text-sm">{r.email}</span>
                                            </div>
                                            <span className={cn("px-2 py-0.5 rounded text-xs border", getStatusColor(r.status))}>
                                                {r.status?.toUpperCase()}
                                            </span>
                                        </div>
                                    ))}
                                </div>
                            ) : (
                                <div className="text-center py-12 text-muted-foreground">
                                    <Upload className="w-12 h-12 mx-auto mb-3 opacity-50" />
                                    <p>Enter emails and click "Verify All" to start</p>
                                </div>
                            )}

                            {bulkResults.length > 0 && (
                                <div className="mt-4 pt-4 border-t border-border">
                                    <div className="grid grid-cols-4 gap-4 text-center">
                                        <div className="bg-green-500/10 rounded-lg p-3">
                                            <div className="text-green-500 font-bold text-xl">
                                                {bulkResults.filter(r => r.status === 'valid' || r.status === 'deliverable').length}
                                            </div>
                                            <div className="text-xs text-muted-foreground">Valid</div>
                                        </div>
                                        <div className="bg-red-500/10 rounded-lg p-3">
                                            <div className="text-red-500 font-bold text-xl">
                                                {bulkResults.filter(r => r.status === 'invalid' || r.status === 'undeliverable').length}
                                            </div>
                                            <div className="text-xs text-muted-foreground">Invalid</div>
                                        </div>
                                        <div className="bg-yellow-500/10 rounded-lg p-3">
                                            <div className="text-yellow-500 font-bold text-xl">
                                                {bulkResults.filter(r => r.status === 'risky' || r.status === 'unknown').length}
                                            </div>
                                            <div className="text-xs text-muted-foreground">Risky</div>
                                        </div>
                                        <div className="bg-muted rounded-lg p-3">
                                            <div className="text-foreground font-bold text-xl">
                                                {bulkResults.length}
                                            </div>
                                            <div className="text-xs text-muted-foreground">Total</div>
                                        </div>
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
