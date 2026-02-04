import { useState, useEffect } from 'react';
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react';

interface PaginationProps {
    currentPage: number;
    totalPages: number;
    setPage: (page: number) => void;
    disabled?: boolean;
}

const Pagination = ({ currentPage, totalPages, setPage, disabled = false }: PaginationProps) => {
    const [inputVal, setInputVal] = useState(currentPage.toString());

    // Sync input value when page changes externally
    useEffect(() => {
        setInputVal(currentPage.toString());
    }, [currentPage]);

    // Hide if only 1 page
    if (totalPages <= 1) return null;

    if (disabled) {
        return (
            <div className="flex justify-center py-6 text-muted-foreground text-sm italic">
                Pagination disabled
            </div>
        );
    }

    const handleFirst = () => setPage(1);
    const handlePrev = () => setPage(Math.max(1, currentPage - 1));
    const handleNext = () => setPage(Math.min(totalPages, currentPage + 1));
    const handleLast = () => setPage(totalPages);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        const val = parseInt(inputVal);
        if (!isNaN(val) && val >= 1 && val <= totalPages) {
            setPage(val);
        } else {
            setInputVal(currentPage.toString()); // Reset to current if invalid
        }
    };

    // Darker text color (text-foreground) for better visibility in light mode
    const buttonClass = "p-2 rounded-md border border-border bg-card hover:bg-secondary disabled:opacity-40 disabled:cursor-not-allowed transition-colors text-foreground";

    return (
        <div className="flex flex-wrap justify-center items-center gap-1.5 py-6 select-none">
            {/* First & Prev */}
            <button onClick={handleFirst} disabled={currentPage === 1} className={buttonClass} title="First Page">
                <ChevronsLeft size={18} />
            </button>
            <button onClick={handlePrev} disabled={currentPage === 1} className={buttonClass} title="Previous Page">
                <ChevronLeft size={18} />
            </button>

            {/* Page Input */}
            <form onSubmit={handleSubmit} className="flex items-center gap-2 mx-1">
                <input
                    type="text"
                    value={inputVal}
                    onChange={(e) => setInputVal(e.target.value)}
                    // Increased to text-base and bold, text-foreground for visibility
                    className="w-12 py-1.5 text-center text-base font-bold bg-secondary border border-transparent focus:border-primary rounded-md outline-none transition-all text-foreground"
                />
                {/* Increased to text-base, darker color */}
                <span className="text-base font-bold text-foreground opacity-80">/ {totalPages}</span>
            </form>

            {/* Next & Last */}
            <button onClick={handleNext} disabled={currentPage === totalPages} className={buttonClass} title="Next Page">
                <ChevronRight size={18} />
            </button>
            <button onClick={handleLast} disabled={currentPage === totalPages} className={buttonClass} title="Last Page">
                <ChevronsRight size={18} />
            </button>
        </div>
    );
};

export default Pagination;