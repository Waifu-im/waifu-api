// HashGen — golden vector generator for the perceptual hash spike.
//
// Produces the reference perceptual hashes that the Go implementation in
// backend/go/internal/images/phash must reproduce bit-for-bit. It calls the
// same library version, in the same way, that the C# service does via
// ImageProcessingService.cs.
//
// Output format — one tab-separated line per image:
//
//	<relative-path>	<hash:X16>	<width>x<height>	<dominantColor>	<isAnimated>
//
// Images that fail to decode are recorded as DECODE_ERROR rather than aborting
// the run, so one bad file cannot block the whole corpus.

using CoenM.ImageHash;
using CoenM.ImageHash.HashAlgorithms;
using SixLabors.ImageSharp;
using SixLabors.ImageSharp.PixelFormats;
using SixLabors.ImageSharp.Processing;

namespace HashGen;

internal static class Program
{
    private static readonly string[] Extensions =
    {
        ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff",
    };

    private static int Main(string[] args)
    {
        // Usage: HashGen <input-dir> <output-file>
        if (args.Length < 2)
        {
            Console.Error.WriteLine("usage: HashGen <input-dir> <output-file>");
            return 1;
        }

        var inputDir = args[0];
        var outputFile = args[1];

        // Enumerate in a stable order so the output file is diffable between runs.
        var files = Directory
            .EnumerateFiles(inputDir, "*", SearchOption.AllDirectories)
            .Where(f => Extensions.Contains(Path.GetExtension(f).ToLowerInvariant()))
            .OrderBy(f => f, StringComparer.Ordinal)
            .ToList();

        if (files.Count == 0)
        {
            Console.Error.WriteLine($"no images found under {inputDir}");
            return 1;
        }

        using var writer = new StreamWriter(outputFile);

        var ok = 0;
        var failed = 0;

        foreach (var file in files)
        {
            var relative = Path.GetRelativePath(inputDir, file).Replace('\\', '/');

            try
            {
                using var stream = File.OpenRead(file);
                using var image = Image.Load<Rgba32>(stream);

                // Mirrors ImageProcessingService.cs exactly:
                //     var hash = _perceptualHash.Hash(image);
                //     hash.ToString("X16");
                var hash = new PerceptualHash().Hash(image);

                // Dominant colour uses the same resize-to-1x1 trick as the C# service.
                using var onePixel = image.Clone(ctx => ctx.Resize(1, 1));
                var px = onePixel[0, 0];
                var dominant = $"#{px.R:X2}{px.G:X2}{px.B:X2}";

                var isAnimated = image.Frames.Count > 1;

                writer.WriteLine(string.Join(
                    '\t',
                    relative,
                    hash.ToString("X16"),
                    $"{image.Width}x{image.Height}",
                    dominant,
                    isAnimated ? "1" : "0"));

                ok++;
            }
            catch (Exception ex)
            {
                writer.WriteLine($"{relative}\tDECODE_ERROR\t{ex.GetType().Name}");
                failed++;
            }
        }

        writer.Flush();
        Console.WriteLine($"HashGen: {ok} hashed, {failed} failed, {files.Count} total");
        return 0;
    }
}