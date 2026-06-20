/** Segmented status filter, matching the review page's status tabs. Generic over the value type. */
export function StatusTabs<T extends string>({ value, onChange, options }: {
    value: T;
    onChange: (v: T) => void;
    options: { value: T; label: string }[];
}) {
    return (
        <div className="flex bg-secondary p-1 rounded-xl overflow-x-auto">
            {options.map(o => (
                <button
                    key={o.value}
                    onClick={() => onChange(o.value)}
                    className={`shrink-0 px-4 py-2 rounded-lg text-sm font-bold whitespace-nowrap transition-all ${value === o.value ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                >
                    {o.label}
                </button>
            ))}
        </div>
    );
}

export default StatusTabs;
