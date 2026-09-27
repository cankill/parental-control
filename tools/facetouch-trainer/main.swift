import Foundation
import CoreGraphics
import CreateML
import ImageIO
import UniformTypeIdentifiers
import Vision

struct Record: Decodable {
    let id: String
    let capturedAt: Date
    let imageFile: String?
    let label: String

    enum CodingKeys: String, CodingKey {
        case id
        case capturedAt = "captured_at"
        case imageFile = "image_file"
        case label
    }
}

enum TrainerError: Error, CustomStringConvertible {
    case usage
    case noImages
    case imageLoad(URL)
    case faceMissing(URL)
    case cropFailed(URL)
    case imageWrite(URL)

    var description: String {
        switch self {
        case .usage:
            return "usage: ifocus-train <face-touch-dataset> <output-directory>"
        case .noImages:
            return "no labeled images found"
        case .imageLoad(let url):
            return "cannot load image: \(url.path)"
        case .faceMissing(let url):
            return "cannot find face: \(url.lastPathComponent)"
        case .cropFailed(let url):
            return "cannot crop image: \(url.lastPathComponent)"
        case .imageWrite(let url):
            return "cannot write image: \(url.path)"
        }
    }
}

private let decoder: JSONDecoder = {
    let decoder = JSONDecoder()
    decoder.dateDecodingStrategy = .iso8601
    return decoder
}()

private func clampedROI(for face: VNFaceObservation) -> CGRect {
    let box = face.boundingBox
    let x = max(0, box.midX - box.width * 0.95)
    let y = max(0, box.minY - box.height * 0.42)
    let maxX = min(1, box.midX + box.width * 0.95)
    let maxY = min(1, box.minY + box.height * 0.72)
    return CGRect(x: x, y: y, width: maxX - x, height: maxY - y)
}

private func cropImage(at sourceURL: URL, to destinationURL: URL) throws {
    guard let source = CGImageSourceCreateWithURL(sourceURL as CFURL, nil),
          let image = CGImageSourceCreateImageAtIndex(source, 0, nil) else {
        throw TrainerError.imageLoad(sourceURL)
    }
    let request = VNDetectFaceRectanglesRequest()
    request.usesCPUOnly = true
    let handler = VNImageRequestHandler(cgImage: image, options: [:])
    try handler.perform([request])
    guard let face = request.results?.max(by: { lhs, rhs in
        lhs.boundingBox.width * lhs.boundingBox.height < rhs.boundingBox.width * rhs.boundingBox.height
    }) else {
        throw TrainerError.faceMissing(sourceURL)
    }
    let roi = clampedROI(for: face)
    let width = CGFloat(image.width)
    let height = CGFloat(image.height)
    let cropRect = CGRect(
        x: roi.minX * width,
        y: (1 - roi.maxY) * height,
        width: roi.width * width,
        height: roi.height * height
    ).integral.intersection(CGRect(x: 0, y: 0, width: width, height: height))
    guard cropRect.width > 1, cropRect.height > 1,
          let cropped = image.cropping(to: cropRect) else {
        throw TrainerError.cropFailed(sourceURL)
    }
    guard let destination = CGImageDestinationCreateWithURL(
        destinationURL as CFURL,
        UTType.jpeg.identifier as CFString,
        1,
        nil
    ) else {
        throw TrainerError.imageWrite(destinationURL)
    }
    CGImageDestinationAddImage(destination, cropped, [
        kCGImageDestinationLossyCompressionQuality: 0.92
    ] as CFDictionary)
    guard CGImageDestinationFinalize(destination) else {
        throw TrainerError.imageWrite(destinationURL)
    }
}

private func stableTestAssignment(_ record: Record) -> Bool {
    // FNV-1a keeps the split stable across runs without relying on Swift's
    // randomized Hasher. A fifth of each label becomes an untouched test set.
    var value: UInt64 = 14695981039346656037
    for byte in record.id.utf8 {
        value ^= UInt64(byte)
        value &*= 1099511628211
    }
    return value % 5 == 0
}

private func prepareDataset(datasetURL: URL, workURL: URL) throws -> (Int, Int) {
    let fileManager = FileManager.default
    try? fileManager.removeItem(at: workURL)
    for split in ["training", "testing"] {
        for label in ["watch", "ignore"] {
            try fileManager.createDirectory(
                at: workURL.appendingPathComponent(split).appendingPathComponent(label),
                withIntermediateDirectories: true
            )
        }
    }

    let metadata = try fileManager.contentsOfDirectory(
        at: datasetURL,
        includingPropertiesForKeys: nil
    ).filter { $0.pathExtension == "json" }.sorted { $0.lastPathComponent < $1.lastPathComponent }

    var trainingCount = 0
    var testingCount = 0
    var skipped = 0
    for metadataURL in metadata {
        let record = try decoder.decode(Record.self, from: Data(contentsOf: metadataURL))
        guard record.label == "watch" || record.label == "ignore",
              let imageFile = record.imageFile else {
            continue
        }
        let sourceURL = datasetURL.appendingPathComponent(imageFile)
        let split = stableTestAssignment(record) ? "testing" : "training"
        let destinationURL = workURL
            .appendingPathComponent(split)
            .appendingPathComponent(record.label)
            .appendingPathComponent(record.id + ".jpg")
        do {
            try cropImage(at: sourceURL, to: destinationURL)
            if split == "testing" {
                testingCount += 1
            } else {
                trainingCount += 1
            }
        } catch {
            skipped += 1
            FileHandle.standardError.write(Data("skip \(record.id): \(error)\n".utf8))
        }
    }
    guard trainingCount > 0, testingCount > 0 else {
        throw TrainerError.noImages
    }
    print("Prepared \(trainingCount) training and \(testingCount) testing crops; skipped \(skipped)")
    return (trainingCount, testingCount)
}

private func train(datasetURL: URL, outputURL: URL) throws {
    let fileManager = FileManager.default
    try fileManager.createDirectory(at: outputURL, withIntermediateDirectories: true)
    let workURL = outputURL.appendingPathComponent("prepared", isDirectory: true)
    _ = try prepareDataset(datasetURL: datasetURL, workURL: workURL)

    let trainingURL = workURL.appendingPathComponent("training", isDirectory: true)
    let testingURL = workURL.appendingPathComponent("testing", isDirectory: true)
    let parameters = MLImageClassifier.ModelParameters(
        featureExtractor: .scenePrint(revision: 1),
        validationData: nil,
        maxIterations: 30,
        augmentationOptions: [.crop, .exposure, .blur]
    )
    let classifier = try MLImageClassifier(
        trainingData: .labeledDirectories(at: trainingURL),
        parameters: parameters
    )
    let testMetrics = classifier.evaluation(on: .labeledDirectories(at: testingURL))
    print("Training metrics: \(classifier.trainingMetrics)")
    print("Validation metrics: \(classifier.validationMetrics)")
    print("Test metrics: \(testMetrics)")

    let modelURL = outputURL.appendingPathComponent("ChinPinchClassifier.mlmodel")
    try? fileManager.removeItem(at: modelURL)
    let metadata = MLModelMetadata(
        author: "iFocus local trainer",
        shortDescription: "Classifies chin-area crops as watch or ignore",
        version: "1.0"
    )
    try classifier.write(to: modelURL, metadata: metadata)
    print("Model: \(modelURL.path)")

    let compiledSource = try MLModel.compileModel(at: modelURL)
    let compiledURL = outputURL.appendingPathComponent("ChinPinchClassifier.mlmodelc", isDirectory: true)
    try? fileManager.removeItem(at: compiledURL)
    try fileManager.copyItem(at: compiledSource, to: compiledURL)
    print("Compiled model: \(compiledURL.path)")
}

do {
    guard CommandLine.arguments.count == 3 else {
        throw TrainerError.usage
    }
    try train(
        datasetURL: URL(fileURLWithPath: CommandLine.arguments[1], isDirectory: true),
        outputURL: URL(fileURLWithPath: CommandLine.arguments[2], isDirectory: true)
    )
} catch {
    FileHandle.standardError.write(Data("Training failed: \(error)\n".utf8))
    exit(1)
}
