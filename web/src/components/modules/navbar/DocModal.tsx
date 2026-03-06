'use client';

import { useState, useMemo } from 'react';
import { useTranslations } from 'next-intl';
import { Copy, Check, BookOpen, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useGroupList } from '@/api/endpoints/group';
import { useAPIKeyList } from '@/api/endpoints/apikey';
import { useSettingList, SettingKey } from '@/api/endpoints/setting';
import { motion, AnimatePresence } from 'motion/react';

type ApiType = 'openai-chat' | 'openai-responses' | 'anthropic';

const API_PATHS: Record<ApiType, string> = {
    'openai-chat': '/v1/chat/completions',
    'openai-responses': '/v1/responses',
    'anthropic': '/v1/messages',
};

function generateCurl(baseUrl: string, apiKey: string, model: string, apiType: ApiType): string {
    const path = API_PATHS[apiType];
    const url = `${baseUrl}${path}`;

    if (apiType === 'anthropic') {
        return `curl -X POST '${url}' \\
  -H 'Content-Type: application/json' \\
  -H 'x-api-key: ${apiKey || 'YOUR_API_KEY'}' \\
  -H 'anthropic-version: 2023-06-01' \\
  -d '{
    "model": "${model || 'YOUR_MODEL'}",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'`;
    }

    if (apiType === 'openai-responses') {
        return `curl -X POST '${url}' \\
  -H 'Content-Type: application/json' \\
  -H 'Authorization: Bearer ${apiKey || 'YOUR_API_KEY'}' \\
  -d '{
    "model": "${model || 'YOUR_MODEL'}",
    "input": "Hello!"
  }'`;
    }

    return `curl -X POST '${url}' \\
  -H 'Content-Type: application/json' \\
  -H 'Authorization: Bearer ${apiKey || 'YOUR_API_KEY'}' \\
  -d '{
    "model": "${model || 'YOUR_MODEL'}",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'`;
}

interface DocModalProps {
    isOpen: boolean;
    onClose: () => void;
}

export function DocModal({ isOpen, onClose }: DocModalProps) {
    const t = useTranslations('doc');
    const [apiType, setApiType] = useState<ApiType>('openai-chat');
    const [selectedApiKey, setSelectedApiKey] = useState<string>('');
    const [selectedModel, setSelectedModel] = useState<string>('');
    const [copied, setCopied] = useState(false);

    const { data: groups } = useGroupList();
    const { data: apiKeys } = useAPIKeyList();
    const { data: settings } = useSettingList();

    const baseUrl = useMemo(() => {
        const setting = settings?.find(s => s.key === SettingKey.ApiBaseUrl);
        return setting?.value?.trim() || 'http://localhost:8080';
    }, [settings]);

    const curlCode = useMemo(
        () => generateCurl(baseUrl, selectedApiKey, selectedModel, apiType),
        [baseUrl, selectedApiKey, selectedModel, apiType]
    );

    const handleCopy = async () => {
        try {
            await navigator.clipboard.writeText(curlCode);
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
        } catch {
            // fallback
        }
    };

    return (
        <AnimatePresence>
            {isOpen && (
                <>
                    {/* Backdrop */}
                    <motion.div
                        className="fixed inset-0 bg-black/40 z-50"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        onClick={onClose}
                    />
                    {/* Modal */}
                    <motion.div
                        className="fixed inset-x-4 bottom-4 top-4 md:inset-auto md:left-1/2 md:top-1/2 md:-translate-x-1/2 md:-translate-y-1/2 md:w-[600px] md:max-h-[80vh] z-50 flex flex-col bg-card rounded-3xl border border-border shadow-2xl overflow-hidden"
                        initial={{ opacity: 0, scale: 0.95, y: 20 }}
                        animate={{ opacity: 1, scale: 1, y: 0 }}
                        exit={{ opacity: 0, scale: 0.95, y: 20 }}
                        transition={{ type: 'spring', stiffness: 300, damping: 30 }}
                    >
                        {/* Header */}
                        <div className="flex items-center justify-between p-6 border-b border-border shrink-0">
                            <div className="flex items-center gap-2">
                                <BookOpen className="h-5 w-5 text-primary" />
                                <h2 className="text-lg font-bold text-card-foreground">{t('title')}</h2>
                            </div>
                            <button
                                onClick={onClose}
                                className="p-1.5 rounded-xl text-muted-foreground hover:text-card-foreground hover:bg-muted/50 transition-colors"
                            >
                                <X className="h-5 w-5" />
                            </button>
                        </div>

                        {/* Content */}
                        <div className="flex-1 overflow-y-auto p-6 space-y-5">
                            {/* API 地址 */}
                            <div className="space-y-1">
                                <label className="text-sm font-medium text-muted-foreground">{t('baseUrl')}</label>
                                <div className="font-mono text-sm bg-muted/30 rounded-xl px-3 py-2 text-card-foreground break-all">{baseUrl}</div>
                            </div>

                            {/* 选择 API 类型 */}
                            <div className="space-y-2">
                                <label className="text-sm font-medium text-card-foreground">{t('apiType')}</label>
                                <Select value={apiType} onValueChange={(v) => setApiType(v as ApiType)}>
                                    <SelectTrigger className="rounded-xl">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent className="rounded-xl">
                                        <SelectItem className="rounded-xl" value="openai-chat">{t('typeOpenAIChat')}</SelectItem>
                                        <SelectItem className="rounded-xl" value="openai-responses">{t('typeOpenAIResponses')}</SelectItem>
                                        <SelectItem className="rounded-xl" value="anthropic">{t('typeAnthropic')}</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>

                            {/* 选择 API Key */}
                            <div className="space-y-2">
                                <label className="text-sm font-medium text-card-foreground">{t('apiKey')}</label>
                                <Select value={selectedApiKey} onValueChange={setSelectedApiKey}>
                                    <SelectTrigger className="rounded-xl">
                                        <SelectValue placeholder={t('apiKeyPlaceholder')} />
                                    </SelectTrigger>
                                    <SelectContent className="rounded-xl">
                                        {(apiKeys ?? []).map((key) => (
                                            <SelectItem key={key.id} className="rounded-xl" value={key.api_key}>
                                                {key.name} <span className="text-muted-foreground text-xs">{key.api_key.slice(0, 16)}...</span>
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>

                            {/* 选择分组（模型） */}
                            <div className="space-y-2">
                                <label className="text-sm font-medium text-card-foreground">{t('model')}</label>
                                <Select value={selectedModel} onValueChange={setSelectedModel}>
                                    <SelectTrigger className="rounded-xl">
                                        <SelectValue placeholder={t('modelPlaceholder')} />
                                    </SelectTrigger>
                                    <SelectContent className="rounded-xl">
                                        {(groups ?? []).map((g) => (
                                            <SelectItem key={g.id} className="rounded-xl" value={g.name}>
                                                {g.name}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>

                            {/* curl 代码 */}
                            <div className="space-y-2">
                                <div className="flex items-center justify-between">
                                    <label className="text-sm font-medium text-card-foreground">{t('curlCode')}</label>
                                    <Button
                                        type="button"
                                        size="sm"
                                        variant="ghost"
                                        onClick={handleCopy}
                                        className="h-7 px-2 gap-1 text-xs"
                                    >
                                        {copied ? (
                                            <><Check className="h-3.5 w-3.5 text-green-500" />{t('copied')}</>
                                        ) : (
                                            <><Copy className="h-3.5 w-3.5" />{t('copy')}</>
                                        )}
                                    </Button>
                                </div>
                                <pre className="rounded-xl bg-muted/50 border border-border p-4 text-xs font-mono text-card-foreground overflow-x-auto whitespace-pre-wrap break-all">
                                    {curlCode}
                                </pre>
                            </div>

                            {/* 端点路径说明 */}
                            <div className="rounded-xl border border-border bg-muted/10 p-4 space-y-2">
                                <div className="text-sm font-medium text-card-foreground">{t('endpoints')}</div>
                                <div className="space-y-1 text-xs text-muted-foreground font-mono">
                                    <div><span className="text-primary">POST</span> {baseUrl}/v1/chat/completions — {t('endpointOpenAIChat')}</div>
                                    <div><span className="text-primary">POST</span> {baseUrl}/v1/responses — {t('endpointOpenAIResponses')}</div>
                                    <div><span className="text-primary">POST</span> {baseUrl}/v1/messages — {t('endpointAnthropic')}</div>
                                    <div><span className="text-primary">POST</span> {baseUrl}/v1/embeddings — {t('endpointEmbeddings')}</div>
                                </div>
                            </div>
                        </div>
                    </motion.div>
                </>
            )}
        </AnimatePresence>
    );
}
