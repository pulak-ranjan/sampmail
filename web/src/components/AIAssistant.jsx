import React, { useState, useRef, useEffect } from "react";
import { Bot, Send, X, MessageSquare, RefreshCw, Sparkles, ChevronDown, User } from "lucide-react";
import ReactMarkdown from "react-markdown";
import { sendAIChat, apiRequest } from "../api";
import { cn } from "../lib/utils";

export default function AIAssistant() {
  const [isOpen, setIsOpen] = useState(false);
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState([]);
  const [loading, setLoading] = useState(false);
  const messagesEndRef = useRef(null);

  useEffect(() => {
    if (isOpen) {
      loadHistory();
    }
  }, [isOpen]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, loading, isOpen]);

  const loadHistory = async () => {
    try {
      const data = await apiRequest("/ai/history");
      if (Array.isArray(data) && data.length > 0) {
        setMessages(data);
      } else if (messages.length === 0) {
        setMessages([
          {
            role: "assistant",
            content: "Hello! I am your **SampMail AI Guardian**. \n\nI can help you audit logs, configure SMTP listeners, manage IP blocks, and analyze bounces.\n\nType a command or ask a question to get started."
          }
        ]);
      }
    } catch (e) {
      console.error("Failed to load history:", e);
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!input.trim() || loading) return;

    const newMsg = { role: "user", content: input };
    setMessages(prev => [...prev, newMsg]);
    setInput("");
    setLoading(true);

    try {
      const res = await sendAIChat({ messages: [], new_msg: input });
      setMessages(prev => [...prev, { role: "assistant", content: res.reply }]);
    } catch (err) {
      setMessages(prev => [...prev, { role: "assistant", content: "Error: " + err.message }]);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed bottom-6 right-6 z-50 flex flex-col items-end gap-4 font-sans antialiased">
      {isOpen && (
        <div className="w-[400px] h-[600px] bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 rounded-2xl shadow-2xl flex flex-col animate-in slide-in-from-bottom-10 fade-in duration-300 overflow-hidden ring-1 ring-black/5">
          {/* Header */}
          <div className="px-4 py-3 border-b border-zinc-100 dark:border-zinc-800 bg-white/80 dark:bg-zinc-900/80 backdrop-blur-md flex justify-between items-center sticky top-0 z-10">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-gradient-to-br from-blue-500 to-indigo-600 rounded-lg shadow-sm text-white relative">
                <Bot className="w-5 h-5" />
                <span className="absolute -bottom-1 -right-1 flex h-2.5 w-2.5">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-green-500 border-2 border-white dark:border-zinc-900"></span>
                </span>
              </div>
              <div className="flex flex-col">
                <span className="font-semibold text-sm text-zinc-900 dark:text-zinc-100">AI Guardian</span>
                <span className="text-[10px] text-zinc-500 dark:text-zinc-400 font-medium tracking-wide uppercase">Online • v1.2</span>
              </div>
            </div>
            <div className="flex gap-1">
              <button
                onClick={loadHistory}
                className="p-2 text-zinc-400 hover:text-zinc-600 hover:bg-zinc-100 dark:hover:bg-zinc-800 rounded-md transition-all"
                title="Reload History"
              >
                <RefreshCw className="w-4 h-4" />
              </button>
              <button
                onClick={() => setIsOpen(false)}
                className="p-2 text-zinc-400 hover:text-zinc-600 hover:bg-zinc-100 dark:hover:bg-zinc-800 rounded-md transition-all"
              >
                <ChevronDown className="w-4 h-4" />
              </button>
            </div>
          </div>

          {/* Messages Area */}
          <div className="flex-1 overflow-y-auto p-4 space-y-6 bg-zinc-50/50 dark:bg-zinc-950/50 scroll-smooth">
            {messages.map((m, i) => (
              <div key={i} className={cn("flex gap-3 text-sm group", m.role === "user" ? "flex-row-reverse" : "flex-row")}>
                {/* Avatar */}
                <div className={cn(
                  "w-8 h-8 rounded-full flex items-center justify-center shrink-0 shadow-sm mt-0.5",
                  m.role === "user" ? "bg-zinc-200 dark:bg-zinc-800" : "bg-gradient-to-br from-blue-500 to-indigo-600 text-white"
                )}>
                  {m.role === "user" ? <User className="w-4 h-4 text-zinc-500" /> : <Sparkles className="w-4 h-4" />}
                </div>

                {/* Bubble */}
                <div className={cn(
                  "p-3.5 rounded-2xl max-w-[85%] shadow-sm leading-relaxed relative animate-in zoom-in-95 duration-200 origin-bottom-left",
                  m.role === "user"
                    ? "bg-zinc-900 dark:bg-white text-white dark:text-zinc-900 rounded-tr-sm"
                    : "bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 text-zinc-800 dark:text-zinc-200 rounded-tl-sm"
                )}>
                  <ReactMarkdown
                    className="prose prose-zinc dark:prose-invert prose-sm max-w-none break-words [&>p]:mb-2 [&>p:last-child]:mb-0 [&>ul]:list-disc [&>ul]:pl-4 [&>ol]:list-decimal [&>ol]:pl-4"
                    components={{
                      code: ({ node, inline, className, children, ...props }) => (
                        inline ?
                          <code className="bg-zinc-100 dark:bg-zinc-800 px-1.5 py-0.5 rounded text-xs font-mono text-zinc-800 dark:text-zinc-200 border border-zinc-200 dark:border-zinc-700 font-medium" {...props}>
                            {children}
                          </code> :
                          <block_code {...props}>{children}</block_code>
                      ),
                      pre: ({ node, children, ...props }) => (
                        <div className="relative mt-2 mb-2 group/code">
                          <pre className="bg-zinc-950 dark:bg-black p-3 rounded-lg overflow-x-auto text-xs font-mono text-zinc-300 border border-zinc-800 shadow-inner" {...props}>
                            {children}
                          </pre>
                        </div>
                      ),
                      a: ({ node, children, ...props }) => (
                        <a className="text-blue-600 hover:underline cursor-pointer" {...props}>{children}</a>
                      )
                    }}
                  >
                    {m.content}
                  </ReactMarkdown>
                </div>
              </div>
            ))}

            {loading && (
              <div className="flex gap-3 text-sm animate-in fade-in slide-in-from-bottom-2 duration-300">
                <div className="w-8 h-8 rounded-full bg-gradient-to-br from-blue-500 to-indigo-600 text-white flex items-center justify-center shrink-0 shadow-sm">
                  <Bot className="w-4 h-4" />
                </div>
                <div className="bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 p-4 rounded-2xl rounded-tl-sm shadow-sm flex items-center gap-1.5 h-10">
                  <div className="w-1.5 h-1.5 bg-zinc-400 rounded-full animate-bounce [animation-delay:-0.3s]"></div>
                  <div className="w-1.5 h-1.5 bg-zinc-400 rounded-full animate-bounce [animation-delay:-0.15s]"></div>
                  <div className="w-1.5 h-1.5 bg-zinc-400 rounded-full animate-bounce"></div>
                </div>
              </div>
            )}
            <div ref={messagesEndRef} />
          </div>

          {/* Input Area */}
          <div className="p-4 bg-white dark:bg-zinc-900 border-t border-zinc-100 dark:border-zinc-800">
            <form onSubmit={handleSubmit} className="relative flex items-center group">
              <input
                value={input}
                onChange={e => setInput(e.target.value)}
                placeholder="Ask AI to analyze logs..."
                className="w-full bg-zinc-50 dark:bg-zinc-950 border border-zinc-200 dark:border-zinc-800 rounded-xl h-12 pl-4 pr-12 text-sm focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all outline-none placeholder:text-zinc-400"
              />
              <button
                type="submit"
                disabled={loading || !input}
                className="absolute right-2 p-2 bg-zinc-900 dark:bg-white text-white dark:text-zinc-900 rounded-lg hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed transition-all shadow-sm"
              >
                <div className={cn("transition-transform duration-200", input ? "translate-x-0.5 -translate-y-0.5" : "")}>
                  <Send className="w-4 h-4" />
                </div>
              </button>
            </form>
            <div className="mt-2 text-[10px] text-center text-zinc-400">
              AI can make mistakes. Verify important info.
            </div>
          </div>
        </div>
      )}

      {/* Toggle Button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className={cn(
          "h-14 w-14 rounded-full shadow-xl flex items-center justify-center transition-all duration-300 hover:scale-110 active:scale-95 z-50",
          isOpen
            ? "bg-zinc-800 text-zinc-300 rotate-90"
            : "bg-gradient-to-r from-blue-600 to-indigo-600 text-white hover:shadow-blue-500/25"
        )}
      >
        {isOpen ? <X className="w-6 h-6" /> : <MessageSquare className="w-6 h-6" />}
      </button>
    </div>
  );
}
