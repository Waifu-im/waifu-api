import { HTMLAttributes } from 'react';

interface SkeletonProps extends HTMLAttributes<HTMLDivElement> {
    className?: string;
}

const Skeleton = ({ className = "", ...props }: SkeletonProps) => {
    return (
        <div
            className={`animate-pulse bg-muted rounded-md ${className}`}
            {...props}
        />
    );
};

export default Skeleton;