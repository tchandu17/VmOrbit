"use client";
import { useState } from "react";
import { usePathname } from "next/navigation";
import { X, Search, BookOpen, ChevronRight, ArrowLeft, Lightbulb } from "lucide-react";
import { useUIStore } from "@/store/useUIStore";
import { cn } from "@/lib/utils";
import {
  helpCategories,
  searchHelpArticles,
  getContextualHelp,
  type HelpArticle,
  type HelpCategory,
} from "@/lib/help-docs";

export function HelpPanel() {
  const isOpen = useUIStore((s) => s.helpPanelOpen);
  const close = useUIStore((s) => s.closeHelpPanel);
  const pathname = usePathname();
  const [query, setQuery] = useState("");
  const [selectedArticle, setSelectedArticle] = useState<HelpArticle | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<HelpCategory | null>(null);

  const contextualArticles = getContextualHelp(pathname);
  const searchResults = searchHelpArticles(query);

  function goBack() {
    if (selectedArticle) {
      setSelectedArticle(null);
    } else if (selectedCategory) {
      setSelectedCategory(null);
    }
  }

  if (!isOpen) return null;

  return (
    <>
      {/* Backdrop */}
      <div className="fixed inset-0 bg-black/40 z-[90]" onClick={close} />

      {/* Panel */}
      <div className="fixed right-0 top-0 h-full w-[420px] max-w-[90vw] bg-gray-900 border-l border-gray-800 z-[91] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-4 border-b border-gray-800 shrink-0">
          <div className="flex items-center gap-2">
            {(selectedArticle || selectedCategory) && (
              <button
                onClick={goBack}
                className="p-1 rounded text-gray-400 hover:text-white hover:bg-gray-800 transition-colors"
              >
                <ArrowLeft className="w-4 h-4" />
              </button>
            )}
            <BookOpen className="w-4 h-4 text-blue-400" />
            <h2 className="font-semibold text-white text-sm">
              {selectedArticle ? selectedArticle.title : "Help Center"}
            </h2>
          </div>
          <button
            onClick={close}
            className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Search */}
        {!selectedArticle && (
          <div className="px-4 py-3 border-b border-gray-800">
            <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-gray-800 border border-gray-700">
              <Search className="w-4 h-4 text-gray-500" />
              <input
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search documentation..."
                className="flex-1 bg-transparent text-sm text-white placeholder-gray-500 outline-none"
              />
            </div>
          </div>
        )}

        {/* Content */}
        <div className="flex-1 overflow-y-auto">
          {/* Article View */}
          {selectedArticle && (
            <div className="p-4">
              <div className="prose prose-invert prose-sm max-w-none">
                <div className="text-xs text-blue-400 mb-2 uppercase tracking-wider font-medium">
                  {selectedArticle.category.replace("-", " ")}
                </div>
                <div className="text-gray-300 text-sm leading-relaxed whitespace-pre-wrap">
                  {renderMarkdown(selectedArticle.content)}
                </div>
              </div>
            </div>
          )}

          {/* Category View */}
          {!selectedArticle && selectedCategory && (
            <div className="p-4 space-y-2">
              <p className="text-xs text-gray-500 mb-3">{selectedCategory.description}</p>
              {selectedCategory.articles.map((article) => (
                <button
                  key={article.id}
                  onClick={() => setSelectedArticle(article)}
                  className="w-full flex items-center gap-3 p-3 rounded-lg bg-gray-800/50 hover:bg-gray-800 text-left transition-colors"
                >
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-white truncate">{article.title}</p>
                    <p className="text-xs text-gray-500 truncate mt-0.5">{article.summary}</p>
                  </div>
                  <ChevronRight className="w-4 h-4 text-gray-600 shrink-0" />
                </button>
              ))}
            </div>
          )}

          {/* Search Results */}
          {!selectedArticle && !selectedCategory && query && (
            <div className="p-4 space-y-2">
              {searchResults.length === 0 ? (
                <p className="text-sm text-gray-500 text-center py-8">
                  No results for &ldquo;{query}&rdquo;
                </p>
              ) : (
                searchResults.map((article) => (
                  <button
                    key={article.id}
                    onClick={() => setSelectedArticle(article)}
                    className="w-full flex items-center gap-3 p-3 rounded-lg bg-gray-800/50 hover:bg-gray-800 text-left transition-colors"
                  >
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-white truncate">{article.title}</p>
                      <p className="text-xs text-gray-500 truncate mt-0.5">{article.summary}</p>
                    </div>
                    <ChevronRight className="w-4 h-4 text-gray-600 shrink-0" />
                  </button>
                ))
              )}
            </div>
          )}

          {/* Default: Categories + Contextual */}
          {!selectedArticle && !selectedCategory && !query && (
            <div className="p-4 space-y-4">
              {/* Contextual Help */}
              {contextualArticles.length > 0 && (
                <div>
                  <div className="flex items-center gap-2 mb-2">
                    <Lightbulb className="w-3.5 h-3.5 text-yellow-400" />
                    <p className="text-[10px] font-semibold uppercase tracking-wider text-gray-500">
                      Relevant to this page
                    </p>
                  </div>
                  {contextualArticles.slice(0, 3).map((article) => (
                    <button
                      key={article.id}
                      onClick={() => setSelectedArticle(article)}
                      className="w-full flex items-center gap-3 p-2.5 rounded-lg bg-blue-600/10 border border-blue-500/20 hover:bg-blue-600/15 text-left transition-colors mb-2"
                    >
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-blue-300 truncate">{article.title}</p>
                        <p className="text-xs text-gray-500 truncate mt-0.5">{article.summary}</p>
                      </div>
                      <ChevronRight className="w-4 h-4 text-blue-500/50 shrink-0" />
                    </button>
                  ))}
                </div>
              )}

              {/* All Categories */}
              <div>
                <p className="text-[10px] font-semibold uppercase tracking-wider text-gray-500 mb-2">
                  Browse Topics
                </p>
                <div className="space-y-1.5">
                  {helpCategories.map((category) => (
                    <button
                      key={category.id}
                      onClick={() => setSelectedCategory(category)}
                      className="w-full flex items-center gap-3 p-3 rounded-lg bg-gray-800/30 hover:bg-gray-800 text-left transition-colors"
                    >
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-white">{category.label}</p>
                        <p className="text-xs text-gray-500 mt-0.5">{category.description}</p>
                      </div>
                      <span className="text-[10px] text-gray-600 shrink-0">
                        {category.articles.length} articles
                      </span>
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="px-4 py-3 border-t border-gray-800 shrink-0">
          <p className="text-[10px] text-gray-600 text-center">
            Press <kbd className="bg-gray-800 px-1 py-0.5 rounded text-gray-500">Ctrl+/</kbd> to toggle help
          </p>
        </div>
      </div>
    </>
  );
}

/** Simple markdown-like renderer for help content */
function renderMarkdown(content: string) {
  return content.split("\n").map((line, i) => {
    if (line.startsWith("## ")) {
      return (
        <h2 key={i} className="text-lg font-semibold text-white mt-4 mb-2">
          {line.slice(3)}
        </h2>
      );
    }
    if (line.startsWith("### ")) {
      return (
        <h3 key={i} className="text-base font-medium text-gray-200 mt-3 mb-1.5">
          {line.slice(4)}
        </h3>
      );
    }
    if (line.startsWith("| ")) {
      // Simple table row rendering
      const cells = line.split("|").filter(Boolean).map((c) => c.trim());
      if (cells.every((c) => c.match(/^-+$/))) return null; // separator row
      return (
        <div key={i} className="flex gap-4 text-xs py-1 border-b border-gray-800">
          {cells.map((cell, j) => (
            <span key={j} className={cn("flex-1", j === 0 && "font-medium text-gray-200")}>
              {cell.replace(/\*\*/g, "")}
            </span>
          ))}
        </div>
      );
    }
    if (line.startsWith("- ")) {
      return (
        <li key={i} className="text-gray-400 ml-4 list-disc text-sm">
          {formatInline(line.slice(2))}
        </li>
      );
    }
    if (line.match(/^\d+\. /)) {
      return (
        <li key={i} className="text-gray-400 ml-4 list-decimal text-sm">
          {formatInline(line.replace(/^\d+\. /, ""))}
        </li>
      );
    }
    if (line.trim() === "") {
      return <div key={i} className="h-2" />;
    }
    return (
      <p key={i} className="text-gray-400 text-sm">
        {formatInline(line)}
      </p>
    );
  });
}

function formatInline(text: string) {
  // Bold
  const parts = text.split(/(\*\*[^*]+\*\*)/g);
  return parts.map((part, i) => {
    if (part.startsWith("**") && part.endsWith("**")) {
      return (
        <strong key={i} className="text-gray-200 font-medium">
          {part.slice(2, -2)}
        </strong>
      );
    }
    // Inline code
    const codeParts = part.split(/(`[^`]+`)/g);
    return codeParts.map((cp, j) => {
      if (cp.startsWith("`") && cp.endsWith("`")) {
        return (
          <code key={`${i}-${j}`} className="bg-gray-800 px-1 py-0.5 rounded text-xs text-blue-300">
            {cp.slice(1, -1)}
          </code>
        );
      }
      return <span key={`${i}-${j}`}>{cp}</span>;
    });
  });
}
