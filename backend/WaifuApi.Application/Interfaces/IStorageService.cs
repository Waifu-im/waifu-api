using System.Collections.Generic;
using System.IO;
using System.Threading;
using System.Threading.Tasks;

namespace WaifuApi.Application.Interfaces;

public interface IStorageService
{
    Task UploadAsync(Stream stream, string fileName, string contentType);
    Task DeleteAsync(string fileName);
    Task<List<string>> ListObjectsAsync(CancellationToken ct);
}
